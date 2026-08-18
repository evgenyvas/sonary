import SonaryLitElement from '@/base'
import { html } from 'lit'
import { customElement, property, state } from 'lit/decorators.js'
import { createRef, ref } from 'lit/directives/ref.js'
import { repeat } from 'lit/directives/repeat.js'
import { type Track } from '@/types'
import store, {
    fetchTracks, setProgressIndeterminate, getTracksKey, setTrackItem,
    type RootState, type TracksQuery, fetchTracksMode, setCurrentTracksKey
} from '@/store'
import { formatDynamicTime } from '@/utils/func'
import '@awesome.me/webawesome/dist/components/button/button.js'
import '@awesome.me/webawesome/dist/components/icon/icon.js'
import '@awesome.me/webawesome/dist/components/tooltip/tooltip.js'
import '@awesome.me/webawesome/dist/components/dropdown/dropdown.js'
import '@awesome.me/webawesome/dist/components/dropdown-item/dropdown-item.js'
import '@awesome.me/webawesome/dist/components/dialog/dialog.js'
import { classMap } from 'lit/directives/class-map.js'
import '@/components/cover-dialog'
import '@/components/track-like-button'
import '@/components/convert-manager'
import '@/components/play-manager'

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

    private convertManagerRef = createRef<any>()
    private playManagerRef = createRef<any>()

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
        if (this._items.length === 0) {
            this.dispatchEvent(new CustomEvent('tracks-empty', { bubbles: true, composed: true }));
        } else {
            this.dispatchEvent(new CustomEvent('tracks-loaded', { bubbles: true, composed: true }));
        }
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
        const dialog: any = this.lyricsDialogRef.value!
        dialog.open = true
    }

    private lyricsDialogRef = createRef<any>()

    private _delDialogHide() {
        this._selectedItem = null
        this.store.dispatch(setTrackItem(null))
    }

    private get _showArtist(): boolean {
        return [...new Set(this._items.map((track: Track) => track.artist))].length > 1
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
      <wa-dropdown-item value="convert" @click="${() => this.convertManagerRef.value?.convertBatch(this._items)}">Convert</wa-dropdown-item>
      <wa-dropdown-item value="play" @click="${() => this.playManagerRef.value?.playBatch(this._items)}">Play</wa-dropdown-item>
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
        <span class="wa-flank track-number">
          <span>${item.number}</span>
        </span>
        <wa-button class="play-button-list" appearance="plain" slot="trigger" size="s" variant="neutral" @click="${() => this.playManagerRef.value?.playSingle(item)}">
          <wa-icon name="circle-play" label="Play track" role="img" aria-label="Play track" library="default" rotate="0" style="--rotate-angle: 0deg;"></wa-icon>
        </wa-button>
        <span class="wa-flank">
          <span><a href="${this.baseRoute + item.id}">${item.title}</a></span>
        </span>
      </span>
      ` : html`
      <span class="wa-flank">
        <wa-button class="play-button-list" appearance="plain" slot="trigger" size="s" variant="neutral" @click="${() => this.playManagerRef.value?.playSingle(item)}">
          <wa-icon name="circle-play" label="Play track" role="img" aria-label="Play track" library="default" rotate="0" style="--rotate-angle: 0deg;"></wa-icon>
        </wa-button>
        <span><a href="${this.baseRoute + item.id}">${item.title}</a></span>
      </span>
      <span class="wa-flank">
        <span class="cover-list">
          <sonary-cover-dialog
                  .thumbUrl="${item.cover?.["160"]}"
                  .fullUrl="${item.cover?.["640"]}"
                  .altText="${item.title}"></sonary-cover-dialog>
        </span>
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
        ${this.albumId && this._showArtist ? html`
        <span class="wa-flank">
          <span>${item.artist}</span>
        </span>
        ` : ''}
        ${item.lyrics ?
                html`<span>
          <wa-button id="show-lyrics-${item.id}" appearance="plain" slot="trigger" size="s" variant="neutral" @click="${() => this._viewLyrics(item)}" aria-labelledby="wa-tooltip-lyrics-${item.id}">
            <wa-icon name="music-note-list" label="Lyrics" role="img" aria-label="Lyrics" library="default" rotate="0" style="--rotate-angle: 0deg;"></wa-icon>
          </wa-button>
          <wa-tooltip for="show-lyrics-${item.id}" placement="bottom" without-arrow id="wa-tooltip-lyrics-${item.id}">Lyrics</wa-tooltip>
        </span>` : html`<span><wa-button size="s" style="visibility: hidden;"><wa-icon name="music-note-list" library="default"></wa-icon></wa-button></span>`}
        ${this.albumId ? '' : html`
        <span>${item.year === 0 ? '' : item.year}</span>
        <span>${item.type}</span>
        `}
        <span>
          <sonary-track-like-button
                    .trackId="${item.id}"
                    .isLiked="${item.like}"
                    .listMode="${this.mode}"></sonary-track-like-button>
        </span>
        <span class="wa-caption-s">${formatDynamicTime(item.duration)}</span>
        <wa-dropdown>
          <wa-button appearance="plain" slot="trigger" size="s" variant="neutral">
            <wa-icon name="ellipsis" label="Track Options" role="img" aria-label="Track Options" library="default" rotate="0" style="--rotate-angle: 0deg;"></wa-icon>
          </wa-button>
          <wa-dropdown-item value="convert" @click="${() => this.convertManagerRef.value?.convertSingle(item)}">Convert</wa-dropdown-item>
          <wa-dropdown-item value="play" @click="${() => this.playManagerRef.value?.playSingle(item)}">Play</wa-dropdown-item>
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

  <wa-dialog label="${this._selectedItem?.artist + ' - ' + this._selectedItem?.title}" id="lyrics-view" style="--width: 50vw;" ${ref(this.lyricsDialogRef)} @wa-after-hide="${this._delDialogHide}">
    <pre>${this._selectedItem?.lyrics}</pre>
    <wa-button slot="footer" variant="brand" data-dialog="close">Close</wa-button>
  </wa-dialog>

  <sonary-convert-manager ${ref(this.convertManagerRef)}></sonary-convert-manager>
  <sonary-play-manager ${ref(this.playManagerRef)}></sonary-play-manager>
</div>
`
    }
}

declare global {
    interface HTMLElementTagNameMap {
        'sonary-tracks-list': TracksList
    }
}
