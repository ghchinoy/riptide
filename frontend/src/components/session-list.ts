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
