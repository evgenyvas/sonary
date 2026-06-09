import { LitElement, html } from 'lit'
import { customElement } from 'lit/decorators.js'
import '@/assets/style.scss'
import '@/components/tracks'

@customElement('sonary-app')
export class SonaryApp extends LitElement {
    render() {
        return html`
      <div class="wa-grid">
          <sonary-tracks></sonary-tracks>
      </div>
    `
    }
}

declare global {
    interface HTMLElementTagNameMap {
        'sonary-app': SonaryApp
    }
}
