import SonaryLitElement from '@/base'
import { html } from 'lit'
import { customElement, property, state } from 'lit/decorators.js'
import store, {
  fetchTrack, updateTrack, setProgressIndeterminate,
  type updateTrackParams
} from '@/store'
import type { Track } from '@/types'
import { formatDynamicTime } from '@/utils/func'
import '@awesome.me/webawesome/dist/components/button/button.js'
import '@awesome.me/webawesome/dist/components/icon/icon.js'
import '@awesome.me/webawesome/dist/components/tooltip/tooltip.js'
import '@awesome.me/webawesome/dist/components/dropdown/dropdown.js'
import '@awesome.me/webawesome/dist/components/dropdown-item/dropdown-item.js'
import { classMap } from 'lit/directives/class-map.js'
import { notify } from '@/utils/notifier'

@customElement('sonary-tracks-view')
export class TracksView extends SonaryLitElement {
  @property({ type: Number, attribute: 'id' })
  trackId: number | null = null

  @state()
  private _selectedItem: Track | null = null

  @state()
  private _isLoading: boolean = false

  @property({ type: String, attribute: 'base-route' })
  baseRoute: string = '/'

  connectedCallback() {
    super.connectedCallback()

    if (!this.storeState.tracks.currentKey) {
      this._isLoading = true
      this.store.dispatch(setProgressIndeterminate(true))
    }
    store.dispatch(fetchTrack(<number>this.trackId)).then(() => {
      if (!this.storeState.tracks.currentKey) {
        this._isLoading = false
        this.store.dispatch(setProgressIndeterminate(false))
      }
      this._selectedItem = this.storeState.tracks.selectedItem
    })
  }

  _toggleLike() {
    this._isLoading = true
    this.store.dispatch(setProgressIndeterminate(true))
    let val = !this._selectedItem?.like
    store.dispatch(updateTrack(<number>this.trackId, <updateTrackParams>{
      like: val
    })).then(() => {
      this._isLoading = false
      this.store.dispatch(setProgressIndeterminate(false))
      this._selectedItem = this.storeState.tracks.selectedItem
      if (this._selectedItem?.like) {
        notify('Added to favorites', 'success')
      } else {
        notify('Removed from favorites', 'success')
      }
    })
  }

  render() {
    return this.getErrorMessage() || this._isLoading ? '' : html`
<div>
  <div class="wa-cluster wa-justify-content-end">
    <wa-button @click="${() => this._toggleLike()}" id="favorite" appearance="plain" size="s" variant="neutral" aria-labelledby="wa-tooltip-jIZGIfGHQ-pedYd5lbie9">
      <wa-icon name="heart" label="Favorite" variant="regular" role="img" aria-label="Favorite" library="default" rotate="0" style="--rotate-angle: 0deg;" class="${classMap({ fav_true: <boolean>this._selectedItem?.like })}"></wa-icon>
    </wa-button>
    <wa-tooltip for="favorite" placement="bottom" distance="2" without-arrow id="wa-tooltip-jIZGIfGHQ-pedYd5lbie9">Favorite</wa-tooltip>
    <wa-dropdown>
      <wa-button id="options" appearance="plain" slot="trigger" size="s" variant="neutral" aria-labelledby="wa-tooltip-4JMAo0Oz3lCxM3wujKNlc">
        <wa-icon name="ellipsis" label="Track Options" role="img" aria-label="Track Options" library="default" rotate="0" style="--rotate-angle: 0deg;"></wa-icon>
      </wa-button>
      <wa-dropdown-item value="convert">Convert</wa-dropdown-item>
    </wa-dropdown>
    <wa-tooltip for="options" placement="bottom" distance="2" without-arrow id="wa-tooltip-4JMAo0Oz3lCxM3wujKNlc">Options</wa-tooltip>
  </div>
  <div class="wa-flank wa-gap-3xl" style="--content-percentage: 40%">
    <div class="wa-split:column wa-align-items-start">
      <div class="wa-stack" style="margin-block: auto">
        <h1 class="wa-heading-4xl">${this._selectedItem?.title}</h1>
        <a href="${this.baseRoute + 'artists/' + this._selectedItem?.artist_id}" class="wa-heading-l">${this._selectedItem?.artist}</a>
        <div><a href="${this.baseRoute + 'albums/' + this._selectedItem?.album_id}">${this._selectedItem?.album}</a></div>
        <div>${this._selectedItem?.genre}</div>
        <div class="wa-cluster wa-caption-s wa-gap-2xs">
          <span>${this._selectedItem?.type}</span>
          ${this._selectedItem?.year === 0 ? '' :
        html`<span>•</span>
          <span>${this._selectedItem?.year}</span>`}
          <span>•</span>
          <span>${formatDynamicTime(<number>this._selectedItem?.duration)}</span>
        </div>
        ${this._selectedItem?.lyrics &&
      html`<pre>${this._selectedItem?.lyrics}</pre>`}
      </div>
    </div>
  </div>
</div>
`
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'sonary-tracks-view': TracksView
  }
}
