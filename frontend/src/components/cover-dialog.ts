import SonaryLitElement from '@/base'
import { html, nothing } from 'lit'
import { customElement, property } from 'lit/decorators.js'
import { createRef, ref } from 'lit/directives/ref.js'
import '@awesome.me/webawesome/dist/components/dialog/dialog.js'

@customElement('sonary-cover-dialog')
export class CoverDialog extends SonaryLitElement {
    @property({ type: String }) thumbUrl = ''
    @property({ type: String }) fullUrl = ''
    @property({ type: String }) altText = ''

    private dialogRef = createRef<HTMLElement>()

    private _openDialog(e: Event) {
        e.preventDefault()
        const dialog: any = this.dialogRef.value
        if (dialog) {
            dialog.open = true
        }
    }

    render() {
        return html`
      <a href="#" class="img-cover" @click="${this._openDialog}">
        ${this.thumbUrl ? html`<img src="${this.thumbUrl}" alt="${this.altText}">` : nothing}
      </a>
      <wa-dialog class="img-dialog" light-dismiss="true" ${ref(this.dialogRef)}>
        <div class="wa-text-center">
          ${this.fullUrl ? html`<img src="${this.fullUrl}" alt="${this.altText}">` : nothing}
        </div>
      </wa-dialog>
    `
    }
}

declare global {
    interface HTMLElementTagNameMap {
        'sonary-cover-dialog': CoverDialog
    }
}
