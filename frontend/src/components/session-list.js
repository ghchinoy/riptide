var __decorate = (this && this.__decorate) || function (decorators, target, key, desc) {
    var c = arguments.length, r = c < 3 ? target : desc === null ? desc = Object.getOwnPropertyDescriptor(target, key) : desc, d;
    if (typeof Reflect === "object" && typeof Reflect.decorate === "function") r = Reflect.decorate(decorators, target, key, desc);
    else for (var i = decorators.length - 1; i >= 0; i--) if (d = decorators[i]) r = (c < 3 ? d(r) : c > 3 ? d(target, key, r) : d(target, key)) || r;
    return c > 3 && r && Object.defineProperty(target, key, r), r;
};
import { LitElement, html, css } from 'lit';
import { customElement } from 'lit/decorators.js';
let SessionList = class SessionList extends LitElement {
    render() {
        return html `
      <div class="welcome">
        <md-icon style="--md-icon-size: 64px">history</md-icon>
        <h1>Select a session from the sidebar</h1>
        <p>Review agent reasoning, actions, and screenshots turn-by-turn.</p>
      </div>
    `;
    }
    static { this.styles = css `
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
  `; }
};
SessionList = __decorate([
    customElement('session-list')
], SessionList);
export { SessionList };
