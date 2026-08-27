import SonaryLitElement from '@/base'
import { html } from 'lit'
import { customElement, state, property } from 'lit/decorators.js'
import { type RootState, setSearchQuery } from '@/store'
import '@awesome.me/webawesome/dist/components/icon/icon.js'
import '@awesome.me/webawesome/dist/components/input/input.js'

@customElement('sonary-search-input')
export class SonarySearchInput extends SonaryLitElement {
    @state()
    private _currentSearchQuery: string = ''

    @property({ type: String })
    inputId: string = 'search-header'

    @property({ type: String })
    customClass: string = ''

    @property({ type: String, attribute: 'base-route' })
    baseRoute: string = '/'

    stateChanged(state: RootState): void {
        super.stateChanged(state)
        if (this._currentSearchQuery !== state.app.searchQuery) {
            this._currentSearchQuery = state.app.searchQuery
        }
    }

    private _handleKeyDown(event: KeyboardEvent) {
        if (event.key === 'Enter') {
            event.preventDefault()
            const waInput = event.target as any
            const value = (waInput?.value || '').trim()
            if (value === '') {
                window.history.pushState({}, '', this.baseRoute)
            } else {
                const query = encodeURIComponent(value)
                window.history.pushState({}, '', `${this.baseRoute}search/${query}`)
            }
            window.dispatchEvent(new PopStateEvent('popstate'))
            this.store.dispatch(setSearchQuery(value))
        }
    }

    protected render() {
        return html`
      <wa-input
        id="${this.inputId}"
        class="${this.customClass}"
        placeholder="Search"
        style="max-inline-size: 100%"
        type="text"
        size="m"
        appearance="outlined"
        .value="${this._currentSearchQuery}"
        @keydown="${this._handleKeyDown}"
      >
        <wa-icon
          slot="start"
          name="magnifying-glass"
          aria-hidden="true"
          library="default"
          rotate="0"
          style="--rotate-angle: 0deg;"
        ></wa-icon>
      </wa-input>
    `;
    }
}
