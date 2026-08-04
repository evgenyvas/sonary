import SonaryLitElement from '@/base'
import { html } from 'lit'
import { customElement, property, state } from 'lit/decorators.js'
import { ref, createRef, type Ref } from 'lit/directives/ref.js'
import { repeat } from 'lit/directives/repeat.js'
import { onMessage } from '@/modules/websocket/websocket'
import {
    type Track, type TrackConvert, type ConvertTrackParams,
    EventConvertProgressUpdate, EventConvertTrackProgressUpdate,
    ConvertStatusProcessing, ConvertStatusCompleted, ConvertStatusFailed
} from '@/types'
import store, {
    fetchTracks, setProgressIndeterminate, setTrackItem, updateTrack, getTracksKey,
    type RootState, type updateTrackParams, type TracksQuery, fetchTracksMode,
    setCurrentTracksKey, deleteItem, convertTrack, downloadConvert, playTrack
} from '@/store'
import { formatDynamicTime } from '@/utils/func'
import '@awesome.me/webawesome/dist/components/format-date/format-date.js'
import '@awesome.me/webawesome/dist/components/button/button.js'
import '@awesome.me/webawesome/dist/components/icon/icon.js'
import '@awesome.me/webawesome/dist/components/tooltip/tooltip.js'
import '@awesome.me/webawesome/dist/components/dropdown/dropdown.js'
import '@awesome.me/webawesome/dist/components/dropdown-item/dropdown-item.js'
import '@awesome.me/webawesome/dist/components/dialog/dialog.js'
import '@awesome.me/webawesome/dist/components/progress-bar/progress-bar.js'
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

    @state()
    private _pregapTrack: Track | null = null

    @state()
    private _convertProgress: number = 0

    @state()
    private _convertTracksProgress: { [key: number]: number } = []

    @state()
    private _convertTracks: TrackConvert[] = []

    @state()
    private _convertJobId: number = 0

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

    private _unsubscribeMessage: (() => void) | null = null

    connectedCallback() {
        super.connectedCallback()
        if (this.storeState.tracks.currentKey !== this.queryKey) {
            this._loadItems()
        }

        this._unsubscribeMessage = onMessage((msg) => {
            let eventMsg = JSON.parse(msg)
            if (eventMsg.type === EventConvertProgressUpdate) {
                this._convertProgress = eventMsg.progress
                console.log(eventMsg)
                if (eventMsg.total === eventMsg.processed) {
                    notify('Convert finished successfully', 'success')
                    store.dispatch(downloadConvert(this._convertJobId)).then(() => { })
                }
            } else if (eventMsg.type === EventConvertTrackProgressUpdate) {
                if ([ConvertStatusProcessing, ConvertStatusCompleted].includes(eventMsg.status)) {
                    this._convertTracksProgress = {
                        ...this._convertTracksProgress,
                        [eventMsg.track_id]: eventMsg.progress
                    }
                    if (eventMsg.status === ConvertStatusCompleted) {
                        notify('Track ' + eventMsg.track_title + ' converted successfully', 'success')
                    }
                } else if (eventMsg.status === ConvertStatusFailed) {
                    notify(eventMsg.error, 'danger')
                }
            }
        })
    }

    disconnectedCallback() {
        if (this._unsubscribeMessage) {
            this._unsubscribeMessage()
        }
        super.disconnectedCallback()
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
        const dialog: any = this.viewLyricsDialogRef.value!
        dialog.open = true
    }

    viewLyricsDialogRef: Ref<HTMLInputElement> = createRef()
    pregapDialogRef: Ref<HTMLInputElement> = createRef()
    convertDialogRef: Ref<HTMLInputElement> = createRef()

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

    private get _showArtist(): boolean {
        return [...new Set(this._items.map((track: Track) => track.artist))].length > 1
    }

    private _onConvertClick(track: Track) {
        if (track.pregap && track.pregap_duration > 0) {
            this._pregapTrack = track
            const dialog: any = this.pregapDialogRef.value!
            dialog.open = true
        } else {
            this._executeConvert(track, false)
        }
    }

    private _executeConvert(track: Track, includePregap: boolean) {
        this._convertTracks = [{ id: track.id, title: track.title }]
        this._convertTracksProgress = []
        this._convertTracksProgress[track.id] = 0
        this._sendConvert([track.id], includePregap)
    }

    private _executeConvertBatch() {
        this._convertTracks = []
        this._convertTracksProgress = []
        let trackIds = <number[]>[]
        this._items.forEach((track) => {
            this._convertTracksProgress[track.id] = 0
            this._convertTracks.push({ id: track.id, title: track.title })
            trackIds.push(track.id)
        })
        this._sendConvert(trackIds, false)
    }

    private _sendConvert(trackIds: number[], includePregap: boolean) {
        this._convertProgress = 0
        this._convertJobId = 0
        this.store.dispatch(setProgressIndeterminate(true))
        store.dispatch(convertTrack(trackIds, <ConvertTrackParams>{
            format: 'mp3',
            mode: 'cbr',
            quality: '320',
            include_pregap: includePregap
        })).then((response: any) => {
            if (!response.directDownload) {
                const dialog: any = this.convertDialogRef.value!
                dialog.open = true
                this._convertJobId = response.job_id

                console.log(response)
            }
            this.store.dispatch(setProgressIndeterminate(false))
        })
    }

    private _executePlay(track: Track) {
        this._convertTracks = [{ id: track.id, title: track.title }]
        this._convertTracksProgress = []
        this._convertTracksProgress[track.id] = 0
        this._sendPlay([track.id])
    }

    private _executePlayBatch() {
        let trackIds = <number[]>[]
        this._items.forEach((track) => {
            trackIds.push(track.id)
        })
        this._sendPlay(trackIds)
    }

    private _sendPlay(trackIds: number[]) {
        this.store.dispatch(setProgressIndeterminate(true))
        store.dispatch(playTrack(trackIds)).then(() => {
            this.store.dispatch(setProgressIndeterminate(false))
        })
    }

    private _handlePregapChoice(includePregap: boolean) {
        if (!this._pregapTrack) return
        const dialog: any = this.pregapDialogRef.value!
        dialog.open = false

        this._executeConvert(this._pregapTrack, includePregap)
        this._pregapTrack = null
    }

    private _formatPregapDuration(pregapDuration: number): string {
        if (!pregapDuration) return '00:00'
        const minutes = Math.floor(pregapDuration / 60)
        const seconds = pregapDuration % 60
        return `${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')}`
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
      <wa-dropdown-item value="convert" @click="${() => this._executeConvertBatch()}">Convert</wa-dropdown-item>
      <wa-dropdown-item value="play" @click="${() => this._executePlayBatch()}">Play</wa-dropdown-item>
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
          <wa-dropdown-item value="convert" @click="${() => this._onConvertClick(item)}">Convert</wa-dropdown-item>
          <wa-dropdown-item value="play" @click="${() => this._executePlay(item)}">Play</wa-dropdown-item>
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

  <wa-dialog ${ref(this.pregapDialogRef)} label="Pregap detected">
    <p>
      This track contains a pregap of length
      <strong>${this._formatPregapDuration(this._pregapTrack?.pregap_duration || 0)}</strong>.
      Want to add it to the beginning of the song?
    </p>
    <wa-button slot="footer" variant="neutral" size="s" @click="${() => this._handlePregapChoice(false)}">
      No, cut it off
    </wa-button>
    <wa-button slot="footer" variant="brand" size="s" @click="${() => this._handlePregapChoice(true)}">
      Yes, add
    </wa-button>
  </wa-dialog>

  <wa-dialog ${ref(this.convertDialogRef)} label="Convert" without-header>
    <div>
      <div class="${classMap({ 'wa-visually-hidden': this._convertTracks.length <= 1 })}">
        <b>Convert</b>
        <wa-progress-bar .value="${this._convertProgress}">${this._convertProgress}%</wa-progress-bar>
      </div>
      ${repeat(this._convertTracks, (item: TrackConvert) => item.id, (item: TrackConvert, index: number) => html`
      <b>Convert track ${item.title}</b>
      <wa-progress-bar data-key="${index}" .value="${this._convertTracksProgress[item.id]}">${this._convertTracksProgress[item.id]}%</wa-progress-bar>
      `)}
    </div>
    ${this._convertProgress >= 100 ? html`
    <wa-button slot="footer" variant="brand" size="s" data-dialog="close">Close</wa-button>
    ` : ''}
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
