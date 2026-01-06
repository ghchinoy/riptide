var __decorate = (this && this.__decorate) || function (decorators, target, key, desc) {
    var c = arguments.length, r = c < 3 ? target : desc === null ? desc = Object.getOwnPropertyDescriptor(target, key) : desc, d;
    if (typeof Reflect === "object" && typeof Reflect.decorate === "function") r = Reflect.decorate(decorators, target, key, desc);
    else for (var i = decorators.length - 1; i >= 0; i--) if (d = decorators[i]) r = (c < 3 ? d(r) : c > 3 ? d(target, key, r) : d(target, key)) || r;
    return c > 3 && r && Object.defineProperty(target, key, r), r;
};
var __metadata = (this && this.__metadata) || function (k, v) {
    if (typeof Reflect === "object" && typeof Reflect.metadata === "function") return Reflect.metadata(k, v);
};
import { LitElement, html, css } from 'lit';
import { customElement, state, query } from 'lit/decorators.js';
import { Router } from '@vaadin/router';
// Material components
import '@material/web/icon/icon.js';
import '@material/web/iconbutton/icon-button.js';
import '@material/web/list/list.js';
import '@material/web/list/list-item.js';
import '@material/web/progress/linear-progress.js';
import '@material/web/divider/divider.js';
// Sub-components (to be created)
import './components/session-list';
import './components/session-detail';
let SessionViewerApp = class SessionViewerApp extends LitElement {
    sessions = [];
    loading = false;
    outlet;
    apiBase = '/api/v1';
    router = null;
    async firstUpdated() {
        console.log('session-viewer-app: firstUpdated');
        await this._fetchSessions();
        this._setupRouter();
    }
    _setupRouter() {
        const outlet = this.renderRoot.querySelector('#outlet');
        console.log('Setting up router. Outlet element:', outlet);
        if (!outlet) {
            console.error('Router outlet not found in renderRoot');
            return;
        }
        this.router = new Router(outlet);
        this.router.setRoutes([
            { path: '/', component: 'session-list' },
            { path: '/sessions/:id', component: 'session-detail' },
        ]);
    }
    async _fetchSessions() {
        const url = `${this.apiBase}/sessions`;
        console.log('Fetching sessions from:', url);
        this.loading = true;
        try {
            const resp = await fetch(url);
            if (!resp.ok)
                throw new Error(`HTTP error! status: ${resp.status}`);
            const data = await resp.json();
            this.sessions = data;
            console.log('Sessions updated, count:', this.sessions.length);
            this.requestUpdate();
        }
        catch (err) {
            console.error('Failed to fetch sessions:', err);
        }
        finally {
            this.loading = false;
        }
    }
    render() {
        return html `
      <div class="app-container">
        <header>
          <div class="top-bar">
            <md-icon-button><md-icon>menu</md-icon></md-icon-button>
            <span class="title">Gemini Session Viewer</span>
            <div class="spacer"></div>
            <md-icon-button @click=${this._fetchSessions} ?disabled=${this.loading}>
              <md-icon>refresh</md-icon>
            </md-icon-button>
          </div>
          ${this.loading ? html `<md-linear-progress indeterminate></md-linear-progress>` : ''}
        </header>

        <main>
          <div class="sidebar">
            <md-list>
              ${this.sessions.map(s => html `
                <md-list-item @click=${() => Router.go(`/sessions/${s.id}`)}>
                  <div slot="headline">${s.prompt?.substring(0, 40)}...</div>
                  <div slot="supporting-text">
                    ${new Date(s.timestamp).toLocaleString()}
                    <span class="status-tag ${s.status}">${s.status}</span>
                  </div>
                  <md-icon slot="start">history</md-icon>
                </md-list-item>
                <md-divider></md-divider>
              `)}
            </md-list>
          </div>
          <div id="outlet" class="content"></div>
        </main>
      </div>
    `;
    }
    static styles = css `
    :host {
      --md-sys-color-primary: #6750a4;
      display: block;
      height: 100vh;
      font-family: 'Roboto', sans-serif;
    }
    .app-container {
      display: flex;
      flex-direction: column;
      height: 100%;
    }
    header {
      background: #fff;
      box-shadow: 0 1px 3px rgba(0,0,0,0.1);
      z-index: 2;
    }
    .top-bar {
      display: flex;
      align-items: center;
      padding: 8px 16px;
      height: 56px;
    }
    .title {
      font-size: 20px;
      margin-left: 16px;
    }
    .spacer { flex: 1; }
    main {
      display: flex;
      flex: 1;
      overflow: hidden;
    }
    .sidebar {
      width: 350px;
      border-right: 1px solid #e0e0e0;
      overflow-y: auto;
      background: #fdfbff;
    }
    .content {
      flex: 1;
      overflow-y: auto;
      padding: 24px;
    }
    md-list-item { cursor: pointer; }
    .status-tag {
      font-size: 0.7rem;
      padding: 2px 6px;
      border-radius: 4px;
      margin-left: 8px;
      text-transform: uppercase;
      font-weight: 500;
    }
    .status-tag.active {
      background: #e8f5e9;
      color: #2e7d32;
      border: 1px solid #2e7d32;
    }
    .status-tag.finished {
      background: #f5f5f5;
      color: #757575;
      border: 1px solid #e0e0e0;
    }
  `;
};
__decorate([
    state(),
    __metadata("design:type", Array)
], SessionViewerApp.prototype, "sessions", void 0);
__decorate([
    state(),
    __metadata("design:type", Object)
], SessionViewerApp.prototype, "loading", void 0);
__decorate([
    query('#outlet'),
    __metadata("design:type", HTMLElement)
], SessionViewerApp.prototype, "outlet", void 0);
SessionViewerApp = __decorate([
    customElement('session-viewer-app')
], SessionViewerApp);
export { SessionViewerApp };
