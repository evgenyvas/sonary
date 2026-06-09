import { css, type CSSResultGroup, html, LitElement } from 'lit'
import { state } from 'lit/decorators.js'
import '@awesome.me/webawesome/dist/components/callout/callout.js'
import '@awesome.me/webawesome/dist/components/icon/icon.js'
import errorStore, { type ErrorRootState, setErrorContext, setErrorMessage } from '@/modules/error/store'
import type { Store, Unsubscribe } from 'redux'
import { ErrorShowMode } from '@/modules/error/types'
import appIconLibrary from '@/utils/icon-library'
import { registerIconLibrary } from '@awesome.me/webawesome'
registerIconLibrary(appIconLibrary.name, { resolver: appIconLibrary.resolver })

export default class BaseLitElement extends LitElement {
    public errorStore: Store = errorStore
    public errorStoreState: ErrorRootState = errorStore.getState()

    _errorStoreUnsubscribe!: Unsubscribe

    @state()
    private _errorMessage: string = ''

    @state()
    private _errorShowMode: ErrorShowMode = ErrorShowMode.None

    connectedCallback() {
        super.connectedCallback()
        this._errorStoreUnsubscribe = errorStore.subscribe(() => this.errorStateChanged(this.errorStoreState))
        this.errorStateChanged(this.errorStoreState)
    }

    disconnectedCallback() {
        this._errorStoreUnsubscribe()
        super.disconnectedCallback()
    }

    // error store state changed
    errorStateChanged(errorState: ErrorRootState): void {
        this._errorMessage = errorState.error.errorMsg
        this._errorShowMode = errorState.error.errorShowMode
    }

    public removeError() {
        this.errorStore.dispatch(setErrorMessage(''))
        this.errorStore.dispatch(setErrorContext())
    }

    public getErrorMessage() {
        return (this._errorMessage && this._errorShowMode === ErrorShowMode.Alert)
            ? html`<wa-callout style="display: flex;" variant="danger" open>
                    <wa-icon slot="icon" name="exclamation-octagon" library="app"></wa-icon>
                    ${this._errorMessage}
                </wa-callout>
                <wa-button @click="${this.removeError}" size="small" type="button" variant="default" part="delete-error" class="delete-error">Back</wa-button>`
            : false
    }

    static styles = css`
        .delete-error {
            margin-top: var(--wa-spacing-small);
        }
    ` as CSSResultGroup
}
