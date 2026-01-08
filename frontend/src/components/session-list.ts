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
import { customElement } from 'lit/decorators.js';

@customElement('session-list')
export class SessionList extends LitElement {
  render() {
    return html`
      <div class="welcome">
        <md-icon style="--md-icon-size: 64px">history</md-icon>
        <h1>Select a session from the sidebar</h1>
        <p>Review agent reasoning, actions, and screenshots turn-by-turn.</p>
      </div>
    `;
  }

  static styles = css`
    .welcome {
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      height: 100%;
      color: #49454f;
      text-align: center;
    }
    h1 { margin-top: 24px; font-weight: 400; }
  `;
}
