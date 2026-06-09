import TracksLitElement from './tracks-base'
import { css, html } from 'lit'
import { customElement, property } from 'lit/decorators.js'

@customElement('produman-seller-shops-view')
export class TracksView extends TracksLitElement {
    @property({ type: String, attribute: 'path' })
    path: string | null = null

    render() {
        return this.getErrorMessage() || html`view`
    }

    static styles = [
        TracksLitElement.styles,
        css`
      :host {
        font-family: var(--wa-font-sans);
        font-size: var(--wa-font-size-large);
      }
    `
    ]
}

declare global {
    interface HTMLElementTagNameMap {
        'sonary-tracks-view': TracksView
    }
}
