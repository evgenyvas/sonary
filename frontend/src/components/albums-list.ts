import SonaryLitElement from '@/base'
import { html } from 'lit'
import { customElement, property, state } from 'lit/decorators.js'
import { repeat } from 'lit/directives/repeat.js'
import type { Album } from '@/types'
import store, {
  fetchAlbums, setProgressIndeterminate, fetchAlbumsMode,
  setCurrentAlbumsKey, type AlbumsQuery, getAlbumsKey, type RootState
} from '@/store'
import '@awesome.me/webawesome/dist/components/button/button.js'
import '@awesome.me/webawesome/dist/components/card/card.js'
import { classMap } from 'lit/directives/class-map.js'

@customElement('sonary-albums-list')
export class AlbumsList extends SonaryLitElement {
  @state()
  private _items: Album[] = []

  @state()
  private _page: number | null = null

  @state()
  private _isLoading: boolean = false

  @state()
  private _hasNext: boolean = false

  @property({ type: Number })
  artistId: number | null = null

  @property({ type: Number })
  limit = 300

  @property({ type: String })
  mode: fetchAlbumsMode = fetchAlbumsMode.All

  @property({ type: String, attribute: 'base-route' })
  baseRoute: string = '/'

  private get query(): AlbumsQuery {
    return {
      mode: this.mode,
      artistId: this.artistId ?? undefined,
    };
  }

  private get queryKey(): string {
    return getAlbumsKey(this.query)
  }

  connectedCallback() {
    super.connectedCallback()
    if (this.storeState.albums.currentKey !== this.queryKey) {
      this._loadItems()
    }
  }

  // store state changed
  stateChanged(state: RootState): void {
    super.stateChanged(state)
    this._items = state.albums.items
    this._hasNext = state.albums.hasNext
  }

  _loadItems() {
    this._isLoading = true
    this.store.dispatch(setCurrentAlbumsKey(this.queryKey))
    this.store.dispatch(setProgressIndeterminate(true))
    store.dispatch(fetchAlbums(this.query, this.limit, this._page)).then(() => {
      this._isLoading = false
      this.store.dispatch(setProgressIndeterminate(false))
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
  <div class="${classMap({
      'items-list-grid': this.artistId === null,
      'wa-split:column wa-align-items-start': this.artistId !== null,
    })}">
    ${this.artistId ? html`
    <ol class="wa-stack wa-gap-0">
      ${repeat(this._items, (item: Album) => item.id, (item: Album, index: number) => html`
      <li class="wa-cluster" data-key="${index}">
        <span class="wa-flank">
          <span>${item.year}</span>
        </span>
        <span class="wa-flank">
          <span><a href="${this.baseRoute + 'albums/' + item.id}">${item.title}</a></span>
        </span>
      </li>
      `)}
    </ol>
    ` :
          repeat(this._items, (item: Album) => item.id, (item: Album, index: number) => html`
    <a class="hover-grow hover-emphasize-border" href="${this.baseRoute + "albums/" + item.id}">
      <wa-card appearance="outlined" orientation="vertical" data-key="${index}">
        <span class="wa-flank">
          ${item.year === 0 ? html`
          <span>${item.artist} - ${item.title}</span>
          ` : html`
          <span>${item.artist} - (${item.year}) ${item.title}</span>
          `}
        </span>
      </wa-card>
    </a>
    `)
        }
  </div>
  ${this._hasNext ? html`<wa-button @click="${() => this._loadMore()}" size="m" style="width: 100%;">Load more</wa-button>` : ''}
  ` : (this._isLoading ? '' : html`<p class="empty-msg">No albums</p>`)}
  ${this._isLoading ? html`<p>Loading albums...</p>` : ''}
</div>
`
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'sonary-albums-list': AlbumsList
  }
}
