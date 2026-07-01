import SonaryLitElement from '@/base'
import { html } from 'lit'
import { customElement, property, state } from 'lit/decorators.js'
import store, { fetchAlbum, setProgressIndeterminate } from '@/store'
import type { Track, Album } from '@/types'
import { formatDynamicTime } from '@/utils/func'
import '@/components/tracks-list'

@customElement('sonary-albums-view')
export class AlbumsView extends SonaryLitElement {
  @property({ type: Number, attribute: 'id' })
  albumId: number | null = null

  @state()
  private _selectedItem: Album | null = null

  @state()
  private _isLoading: boolean = false

  @property({ type: String, attribute: 'base-route' })
  baseRoute: string = '/'

  private get _totalDuration(): number {
    return (this._selectedItem?.tracks || []).reduce((acc: number, track: Track) => {
      return acc + track.duration
    }, 0)
  }

  private get _genre(): string {
    return [...new Set((this._selectedItem?.tracks || []).map((track: Track) => track.genre))].join(', ')
  }

  private get _type(): string {
    return [...new Set((this._selectedItem?.tracks || []).map((track: Track) => track.type))].join(', ')
  }

  connectedCallback() {
    super.connectedCallback()

    this._isLoading = true
    this.store.dispatch(setProgressIndeterminate(true))
    store.dispatch(fetchAlbum(<number>this.albumId)).then(() => {
      this._isLoading = false
      this.store.dispatch(setProgressIndeterminate(false))
      this._selectedItem = this.storeState.albums.selectedItem
    })
  }

  render() {
    return this.getErrorMessage() || this._isLoading ? '' : html`
<div>
  <div class="wa-flank wa-gap-3xl" style="--content-percentage: 40%">
    <div class="wa-split:column wa-align-items-start">
      <div class="wa-stack" style="margin-block: auto">
        <h1 class="wa-heading-4xl">${this._selectedItem?.title}</h1>
        <a href="${this.baseRoute + 'artists/' + this._selectedItem?.artist_id}" class="wa-heading-l">${this._selectedItem?.artist}</a>
        <div>${this._genre}</div>
        <div class="wa-cluster wa-caption-s wa-gap-2xs">
          <span>${this._type}</span>
          ${this._selectedItem?.year === 0 ? '' :
        html`<span>•</span>
          <span>${this._selectedItem?.year}</span>`}
          <span>•</span>
          <span>${formatDynamicTime(<number>this._totalDuration)}</span>
        </div>
        <sonary-tracks-list .baseRoute="${import.meta.env.VITE_BASE_APP_ROUTE}" .albumId=${this._selectedItem?.id}></sonary-tracks-list>
      </div>
    </div>
  </div>
</div>
`
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'sonary-albums-view': AlbumsView
  }
}
