import TracksLitElement from './tracks-base'
import { css, html } from 'lit'
import { customElement, property, state } from 'lit/decorators.js'
import { repeat } from 'lit/directives/repeat.js'
import type { Track } from '../types'
import store, { fetchTracks, type RootState } from '../store'
import '@awesome.me/webawesome/dist/components/format-date/format-date.js'
import '@awesome.me/webawesome/dist/components/button/button.js'
import '@awesome.me/webawesome/dist/components/icon/icon.js'

@customElement('sonary-tracks-list')
export class TracksList extends TracksLitElement {
    @state()
    private _items: Track[] = []

    @state()
    private _isLoading: boolean = false

    @property({ type: String })
    path: string = '/'

    @property({ type: String, attribute: 'base-route' })
    baseRoute: string = '/'

    connectedCallback() {
        super.connectedCallback()
        if (!this.storeState.tracks.isInitList) {
            this._loadItems()
        }
    }

    // store state changed
    stateChanged(state: RootState): void {
        super.stateChanged(state)
        this._items = state.tracks.items
    }

    _loadItems() {
        this._isLoading = true
        store.dispatch(fetchTracks(this.path)).then(() => {
            this._isLoading = false
            if (!this.storeState.tracks.isInitList) {
                this.storeState.tracks.isInitList = true
            }
        })
    }

    render() {
        return this.getErrorMessage() || html`
      <div>
      ${this._items.length > 0 ? html`
        <table>
          <thead>
            <tr>
              <th>Name</th>
              <th>Date</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
          ${repeat(this._items, (item: Track) => item.path, (item: Track, index: number) => html`
            <tr data-key="${index}">
              <td><a href="${this.baseRoute + item.path}">${item.name}</a></td>
              <td><wa-format-date lang="ru" month="numeric" day="numeric" year="numeric" date="${item.createdAt}"></wa-format-date></td>
              <td>
                <wa-button slot="trigger" size="small" circle>
                  <wa-icon name="three-dots" library="app"></wa-icon>
                </wa-button>
              </td>
            </tr>
          `)}
          </tbody>
        </table>
      ` : (this._isLoading ? '' : html`<p class="empty-msg">No tracks</p>`)}
        ${this._isLoading ? html`<p>Loading tracks...</p>` : ''}
      </div>
    `
    }

    static styles = [
        TracksLitElement.styles,
        css`
      :host {
        font-family: var(--wa-font-sans);
        font-size: var(--wa-font-size-large);
      }

      a {
        color: var(--wa-color-primary-600)
      }

      .load-more-button {
        margin-top: var(--wa-spacing-small);
      }

      .empty-msg {
        font-size: var(--wa-font-size-small);
      }
    `
    ]
}

declare global {
    interface HTMLElementTagNameMap {
        'sonary-tracks-list': TracksList
    }
}
