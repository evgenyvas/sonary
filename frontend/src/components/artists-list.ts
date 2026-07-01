import SonaryLitElement from '@/base'
import { html } from 'lit'
import { customElement, property, state } from 'lit/decorators.js'
import { repeat } from 'lit/directives/repeat.js'
import type { Artist } from '@/types'
import store, { fetchArtists, setProgressIndeterminate, type RootState } from '@/store'
import '@awesome.me/webawesome/dist/components/button/button.js'
import '@awesome.me/webawesome/dist/components/card/card.js'

@customElement('sonary-artists-list')
export class ArtistsList extends SonaryLitElement {
  @state()
  private _items: Artist[] = []

  @state()
  private _page: number | null = null

  @state()
  private _isLoading: boolean = false

  @state()
  private _hasNext: boolean = false

  @property({ type: Number })
  limit = 300

  @property({ type: String, attribute: 'base-route' })
  baseRoute: string = '/'

  connectedCallback() {
    super.connectedCallback()
    if (!this.storeState.artists.isInitList) {
      this._loadItems()
    }
  }

  // store state changed
  stateChanged(state: RootState): void {
    super.stateChanged(state)
    this._items = state.artists.items
    this._hasNext = state.artists.hasNext
  }

  _loadItems() {
    this._isLoading = true
    this.store.dispatch(setProgressIndeterminate(true))
    store.dispatch(fetchArtists(this.limit, this._page)).then(() => {
      this._isLoading = false
      this.store.dispatch(setProgressIndeterminate(false))
      if (!this.storeState.artists.isInitList) {
        this.storeState.artists.isInitList = true
      }
    })
  }

  _loadMore() {
    if (!this._page) {
      this._page = 1
    }
    this._page++
    this._loadItems()
  }

  render() {
    return this.getErrorMessage() || html`
<div>
  ${this._items.length > 0 ? html`
  <div class="wa-cluster"><br/></div>
  <div class="items-list-grid">
    ${repeat(this._items, (item: Artist) => item.id, (item: Artist, index: number) => html`
    <a class="hover-grow hover-emphasize-border" href="${this.baseRoute + "artists/" + item.id}">
      <wa-card appearance="outlined" orientation="vertical" data-key="${index}">
        <span class="wa-flank">
          <span>${item.name}</span>
        </span>
      </wa-card>
    </a>
    `)}
  </div>
  ${this._hasNext ? html`<wa-button @click="${() => this._loadMore()}" size="m" style="width: 100%;">Load more</wa-button>` : ''}
  ` : (this._isLoading ? '' : html`<p class="empty-msg">No artists</p>`)}
  ${this._isLoading ? html`<p>Loading artists...</p>` : ''}
</div>
`
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'sonary-artists-list': ArtistsList
  }
}
