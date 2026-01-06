import { LitElement, html, css } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';

@customElement('session-detail')
export class SessionDetail extends LitElement {
  @property({ type: Object }) location?: any;
  @state() session: any = null;
  @state() loading = false;

  private apiBase = '/api/v1';

  async updated(changedProps: Map<string, any>) {
    if (changedProps.has('location')) {
      const id = this.location.params.id;
      await this._fetchSession(id);
    }
  }

  private async _fetchSession(id: string) {
    this.loading = true;
    try {
      const resp = await fetch(`${this.apiBase}/sessions/${id}`);
      this.session = await resp.json();
    } catch (err) {
      console.error('Failed to fetch session detail', err);
    } finally {
      this.loading = false;
    }
  }

  render() {
    if (!this.session) return html`<div>Loading session...</div>`;

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
                    <img src="${this.apiBase}/screenshots/${t.screenshot}" alt="Post-action screenshot">
                    <div class="label">Viewport</div>
                  </div>
                  <div class="screenshot-container">
                    <img src="${this.apiBase}/screenshots/${t.full_page}" alt="Full page screenshot" @error=${(e: any) => e.target.style.display='none'}>
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
