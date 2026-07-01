import SonaryLitElement from '@/base'
import { html } from 'lit'
import { customElement, property, state } from 'lit/decorators.js'
import { ref, createRef, type Ref } from 'lit/directives/ref.js'
import { repeat } from 'lit/directives/repeat.js'
import type { Track } from '@/types'
import store, {
  fetchTracks, setProgressIndeterminate, setTrackItem, updateTrack, getTracksKey,
  type RootState, type updateTrackParams, type TracksQuery, fetchTracksMode,
  setCurrentTracksKey, deleteItem
} from '@/store'
import { formatDynamicTime } from '@/utils/func'
import '@awesome.me/webawesome/dist/components/format-date/format-date.js'
import '@awesome.me/webawesome/dist/components/button/button.js'
import '@awesome.me/webawesome/dist/components/icon/icon.js'
import '@awesome.me/webawesome/dist/components/tooltip/tooltip.js'
import '@awesome.me/webawesome/dist/components/dropdown/dropdown.js'
import '@awesome.me/webawesome/dist/components/dropdown-item/dropdown-item.js'
import '@awesome.me/webawesome/dist/components/dialog/dialog.js'
import { classMap } from 'lit/directives/class-map.js'
import { notify } from '@/utils/notifier'

@customElement('sonary-tracks-list')
export class TracksList extends SonaryLitElement {
  @state()
  private _items: Track[] = []

  @state()
  private _page: number | null = null

  @state()
  private _selectedItem: Track | null = null

  @state()
  private _isLoading: boolean = false

  @state()
  private _hasNext: boolean = false

  @property({ type: Number })
  artistId: number | null = null

  @property({ type: Number })
  albumId: number | null = null

  @property({ type: Number })
  limit = 50

  @property({ type: String })
  mode: fetchTracksMode = fetchTracksMode.All

  @property({ type: String, attribute: 'base-route' })
  baseRoute: string = '/'

  private get query(): TracksQuery {
    return {
      mode: this.mode,
      artistId: this.artistId ?? undefined,
      albumId: this.albumId ?? undefined,
    }
  }

  private get queryKey(): string {
    return getTracksKey(this.query)
  }

  connectedCallback() {
    super.connectedCallback()
    if (this.storeState.tracks.currentKey !== this.queryKey) {
      this._loadItems()
    }
  }

  // store state changed
  stateChanged(state: RootState): void {
    super.stateChanged(state)
    this._items = state.tracks.items
    this._hasNext = state.tracks.hasNext
  }

  _loadItems() {
    this._isLoading = true
    this.store.dispatch(setCurrentTracksKey(this.queryKey))
    this.store.dispatch(setProgressIndeterminate(true))
    store.dispatch(fetchTracks(this.query, this.limit, this._page)).then(() => {
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

  _viewLyrics(track: Track) {
    this._selectedItem = { ...track }
    const dialog: any = this.viewLyricsDialogRef.value!
    dialog.open = true
  }

  viewLyricsDialogRef: Ref<HTMLInputElement> = createRef()

  private _delDialogHide() {
    this._selectedItem = null
    this.store.dispatch(setTrackItem(null))
  }

  _toggleLike(track: Track) {
    this._isLoading = true
    this.store.dispatch(setProgressIndeterminate(true))
    let val = !track.like
    store.dispatch(updateTrack(<number>track.id, <updateTrackParams>{
      like: val
    })).then(() => {
      this._isLoading = false
      this.store.dispatch(setProgressIndeterminate(false))
      if (val) {
        notify('Added to favorites', 'success')
      } else {
        notify('Removed from favorites', 'success')
        if (this.mode === fetchTracksMode.Favorites) {
          this.store.dispatch(deleteItem(track.id))
        }
      }
    })
  }

  render() {
    return this.getErrorMessage() || html`
<div>
  ${this._items.length > 0 ? html`
  <div class="wa-cluster wa-justify-content-end">
    <wa-dropdown>
      <wa-button id="options" appearance="plain" slot="trigger" size="s" variant="neutral" aria-labelledby="wa-tooltip-4JMAo0Oz3lCxM3wujKNlc">
        <wa-icon name="ellipsis" label="Options" role="img" aria-label="Options" library="default" rotate="0" style="--rotate-angle: 0deg;"></wa-icon>
      </wa-button>
      <wa-dropdown-item value="convert">Convert</wa-dropdown-item>
    </wa-dropdown>
    <wa-tooltip for="options" placement="bottom" distance="2" without-arrow id="wa-tooltip-4JMAo0Oz3lCxM3wujKNlc">Options</wa-tooltip>
  </div>
  <ol class="wa-stack wa-gap-0">
    ${repeat(this._items, (item: Track) => item.id, (item: Track, index: number) => html`
    <li class="${classMap({
      'wa-grid': this.albumId === null,
      'wa-cluster wa-justify-content-space-between': this.albumId !== null,
    })}" data-key="${index}">
      ${this.albumId ? html`
      <span class="wa-cluster">
        <span class="wa-flank">
          <span>${item.number}</span>
        </span>
        <span class="wa-flank">
          <span><a href="${this.baseRoute + item.id}">${item.title}</a></span>
        </span>
      </span>
      ` : html`
      <span class="wa-flank">
        <span><a href="${this.baseRoute + item.id}">${item.title}</a></span>
      </span>
      <span class="wa-flank">
        <span>${item.artist}</span>
      </span>
      <span class="wa-flank">
        <span>${item.album}</span>
      </span>
      <span class="wa-flank">
        <span>${item.genre}</span>
      </span>
      `}
      <span class="wa-cluster wa-justify-content-end">
        ${item.lyrics &&
      html`<span>
          <wa-button id="show-lyrics-${item.id}" appearance="plain" slot="trigger" size="s" variant="neutral" @click="${() => this._viewLyrics(item)}" aria-labelledby="wa-tooltip-lyrics-${item.id}">
            <wa-icon name="music-note-list" label="Lyrics" role="img" aria-label="Lyrics" library="default" rotate="0" style="--rotate-angle: 0deg;"></wa-icon>
          </wa-button>
          <wa-tooltip for="show-lyrics-${item.id}" placement="bottom" without-arrow id="wa-tooltip-lyrics-${item.id}">Lyrics</wa-tooltip>
        </span>`}
        ${this.albumId ? '' : html`
        <span>${item.year === 0 ? '' : item.year}</span>
        <span>${item.type}</span>
        `}
        <span>
          <wa-button @click="${() => this._toggleLike(item)}" id="favorite-${item.id}" appearance="plain" size="s" variant="neutral" aria-labelledby="wa-tooltip-like-${item.id}">
            <wa-icon name="heart" label="Favorite" variant="regular" role="img" aria-label="Favorite" library="default" rotate="0" style="--rotate-angle: 0deg;" class="${classMap({ fav_true: item.like })}"></wa-icon>
          </wa-button>
          <wa-tooltip for="favorite-${item.id}" placement="bottom" distance="2" without-arrow id="wa-tooltip-like-${item.id}">Favorite</wa-tooltip>
        </span>
        <span class="wa-caption-s">${formatDynamicTime(item.duration)}</span>
        <wa-dropdown>
          <wa-button appearance="plain" slot="trigger" size="s" variant="neutral">
            <wa-icon name="ellipsis" label="Track Options" role="img" aria-label="Track Options" library="default" rotate="0" style="--rotate-angle: 0deg;"></wa-icon>
          </wa-button>
          <wa-dropdown-item value="convert">Convert</wa-dropdown-item>
          ${item.lyrics &&
      html`<wa-dropdown-item value="lyrics" @click="${() => this._viewLyrics(item)}">View lyrics</wa-dropdown-item>`}
        </wa-dropdown>
      </span>
    </li>
    `)}
  </ol>
  ${this._hasNext ? html`<wa-button @click="${() => this._loadMore()}" size="m" style="width: 100%;">Load more</wa-button>` : ''}
  ` : (this._isLoading ? '' : html`<p class="empty-msg">No tracks</p>`)}
  ${this._isLoading ? html`<p>Loading tracks...</p>` : ''}

  <wa-dialog label="${this._selectedItem?.artist + ' - ' + this._selectedItem?.title}" id="lyrics-view" style="--width: 50vw;" ${ref(this.viewLyricsDialogRef)} @wa-after-hide="${this._delDialogHide}">
    <pre>${this._selectedItem?.lyrics}</pre>
    <wa-button slot="footer" variant="brand" data-dialog="close">Close</wa-button>
  </wa-dialog>

</div>
`
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'sonary-tracks-list': TracksList
  }
}
