/**
 * Copyright 2026 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { LitElement, html, css } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';

@customElement('session-detail')
export class SessionDetail extends LitElement {
  @property({ type: Object }) location?: any;
  @state() session: any = null;
  @state() loading = false;
  @state() error = '';

  private apiBase = '/api/v1';

  private ws: WebSocket | null = null;
  private currentTurnBuffer: any = null;

  async firstUpdated() {
    if (this.location?.params?.id) {
      await this._fetchSession(this.location.params.id);
    }
  }

  async updated(changedProps: Map<string, any>) {
    if (changedProps.has('location') && this.location?.params?.id) {
      if (this.session && this.session.id !== this.location.params.id) {
        this._closeWebSocket();
        await this._fetchSession(this.location.params.id);
      }
    }
  }

  disconnectedCallback() {
    super.disconnectedCallback();
    this._closeWebSocket();
  }

  private _closeWebSocket() {
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
  }

  private async _fetchSession(id: string) {
    console.log('Fetching session:', id);
    this.loading = true;
    this.session = null;
    this.error = '';
    try {
      const resp = await fetch(`${this.apiBase}/sessions/${id}`);
      if (!resp.ok) {
        throw new Error(`Failed to fetch session: ${resp.status} ${resp.statusText}`);
      }
      this.session = await resp.json();
      console.log('Session loaded:', this.session);

      // Initialize Turn Buffer based on latest state if any
      if (this.session && this.session.turns && this.session.turns.length > 0) {
        this.currentTurnBuffer = { ...this.session.turns[this.session.turns.length - 1] };
      } else {
        this.currentTurnBuffer = null;
      }

      this._connectWebSocket(id);

    } catch (err: any) {
      console.error('Failed to fetch session detail', err);
      this.error = err.toString();
    } finally {
      this.loading = false;
    }
  }

  private _connectWebSocket(id: string) {
    this._closeWebSocket();

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    // Use the explicit API backend port (8083) if we're running dev server on 5173
    const host = window.location.port === '5173' ? 'localhost:8083' : window.location.host;
    const wsUrl = `${protocol}//${host}${this.apiBase}/sessions/${id}/stream`;
    
    console.log('Connecting to WebSocket:', wsUrl);
    this.ws = new WebSocket(wsUrl);

    this.ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        this._handleLiveEvent(data);
      } catch (e) {
        console.error("Failed to parse websocket message", e);
      }
    };

    this.ws.onclose = () => {
      console.log('WebSocket disconnected');
    };
  }

  private _handleLiveEvent(event: any) {
    if (!this.session) return;
    
    // We only care about specific events to build up our turn buffer
    if (event.Type === "status" && event.Message.startsWith("Turn ")) {
      // Parse turn index
      const match = event.Message.match(/Turn (\d+)\//);
      if (match) {
        const idx = parseInt(match[1], 10);
        
        // If we have an existing buffer that is older, push it to the main turns array
        if (this.currentTurnBuffer && this.currentTurnBuffer.index !== idx) {
          if (!this.session.turns) this.session.turns = [];
          
          // Check if it already exists (from initial fetch)
          const exists = this.session.turns.findIndex((t: any) => t.index === this.currentTurnBuffer.index);
          if (exists >= 0) {
             this.session.turns[exists] = { ...this.currentTurnBuffer };
          } else {
             this.session.turns.push({ ...this.currentTurnBuffer });
          }
        }
        
        // Start a new buffer
        this.currentTurnBuffer = {
          index: idx,
          thinking: [],
          action: "Thinking...",
          screenshot: `screenshots/turn_${idx}_post.png`,
          full_page: `screenshots/turn_${idx}_full.png`,
        };
        // Trigger a re-render
        this.session = { ...this.session };
      }
    } else if (event.Type === "thinking" && this.currentTurnBuffer) {
      this.currentTurnBuffer.thinking.push(event.Message);
      this.session = { ...this.session }; // trigger render
    } else if (event.Type === "action" && this.currentTurnBuffer) {
      this.currentTurnBuffer.action = event.Message;
      this.session = { ...this.session }; // trigger render
    }
  }

  render() {
    if (this.loading) return html`<div class="loading">Loading session ${this.location?.params?.id}...</div>`;
    if (!this.session) return html`
      <div class="error">
        Session not found or failed to load.
        <div style="color: red; font-weight: bold; margin: 10px 0;">${this.error}</div>
        <pre style="text-align: left; background: #eee; padding: 10px; margin-top: 10px;">
Debug Info:
Location Params: ${JSON.stringify(this.location?.params, null, 2)}
Session: ${JSON.stringify(this.session, null, 2)}
        </pre>
      </div>`;

    return html`
      <div class="session-container">
        <div class="header">
          <h2>Session ${this.session.id}</h2>
          <p class="prompt"><strong>Prompt:</strong> ${this.session.prompt}</p>
        </div>

        <div class="turns">
          ${this.session.turns?.map((t: any) => html`
            <div class="turn-card">
              <div class="turn-header">
                <span class="turn-index">Turn ${t.index}</span>
                <span class="turn-action">${t.action}</span>
              </div>
              <div class="turn-content">
                <div class="reasoning">
                  ${t.thinking?.map((thought: string) => html`<p class="thought">${thought}</p>`)}
                </div>
                <div class="visuals">
                  <div class="screenshot-container">
                    <img src="${this.apiBase}/sessions/${this.session.id}/${t.screenshot}" alt="Post-action screenshot">
                    <div class="label">Viewport</div>
                  </div>
                  <div class="screenshot-container">
                    <img src="${this.apiBase}/sessions/${this.session.id}/${t.full_page}" alt="Full page screenshot" @error=${(e: any) => e.target.style.display='none'}>
                    <div class="label">Full Page</div>
                  </div>
                </div>
              </div>
            </div>
          `)}
        </div>
      </div>
    `;
  }

  static styles = css`
    :host { display: block; padding: 24px; }
    .loading, .error { 
      display: flex; 
      justify-content: center; 
      align-items: center; 
      height: 200px; 
      color: #49454f;
      font-style: italic;
    }
    .error { color: #b3261e; }
    .session-container {
      max-width: 1200px;
      margin: 0 auto;
    }
    .header { margin-bottom: 32px; border-bottom: 1px solid #e0e0e0; padding-bottom: 16px; }
    h2 { margin: 0; font-weight: 400; color: #1d1b20; }
    .prompt { color: #49454f; font-size: 0.9rem; line-height: 1.5; }
    
    .turns { display: flex; flex-direction: column; gap: 32px; }
    .turn-card {
      background: #fff;
      border: 1px solid #e0e0e0;
      border-radius: 12px;
      overflow: hidden;
      display: flex;
      flex-direction: column;
    }
    .turn-header {
      background: #f3edf7;
      padding: 12px 16px;
      display: flex;
      justify-content: space-between;
      align-items: center;
      border-bottom: 1px solid #e0e0e0;
    }
    .turn-index { font-weight: 500; color: #6750a4; }
    .turn-action { font-family: monospace; font-size: 0.85rem; color: #1d1b20; }
    
    .turn-content { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; padding: 16px; }
    .reasoning { font-size: 0.9rem; color: #49454f; line-height: 1.6; }
    .thought { margin-bottom: 12px; }
    
    .visuals { display: flex; flex-direction: column; gap: 16px; }
    .screenshot-container { 
      border: 1px solid #e0e0e0; 
      border-radius: 8px; 
      overflow: hidden;
      position: relative;
    }
    img { width: 100%; display: block; object-fit: contain; }
    .label {
      position: absolute;
      top: 8px;
      right: 8px;
      background: rgba(0,0,0,0.6);
      color: #fff;
      padding: 2px 8px;
      border-radius: 4px;
      font-size: 0.7rem;
      text-transform: uppercase;
    }
  `;
}
