import SonaryLitElement from '@/base'
import { html } from 'lit'
import { customElement, property, state } from 'lit/decorators.js'
import { createRef, ref } from 'lit/directives/ref.js'
import store, {
    fetchTrack, setProgressIndeterminate, type RootState
} from '@/store'
import type { Track } from '@/types'
import { formatDynamicTime } from '@/utils/func'
import '@awesome.me/webawesome/dist/components/button/button.js'
import '@awesome.me/webawesome/dist/components/icon/icon.js'
import '@awesome.me/webawesome/dist/components/tooltip/tooltip.js'
import '@awesome.me/webawesome/dist/components/dropdown/dropdown.js'
import '@awesome.me/webawesome/dist/components/dropdown-item/dropdown-item.js'
import '@/components/cover-dialog'
import '@/components/track-like-button'
import '@/components/convert-manager'
import '@/components/play-manager'

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

    private convertManagerRef = createRef<any>()
    private playManagerRef = createRef<any>()

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

    stateChanged(state: RootState): void {
        super.stateChanged(state)
        this._selectedItem = state.tracks.selectedItem
    }

    render() {
        return this.getErrorMessage() || this._isLoading ? '' : html`
<div>
  <div class="wa-cluster wa-justify-content-end">
    <sonary-track-like-button
            .trackId="${this._selectedItem?.id}"
            .isLiked="${this._selectedItem?.like}"></sonary-track-like-button>
    <wa-dropdown>
      <wa-button id="options" appearance="plain" slot="trigger" size="s" variant="neutral" aria-labelledby="wa-tooltip-4JMAo0Oz3lCxM3wujKNlc">
        <wa-icon name="ellipsis" label="Track Options" role="img" aria-label="Track Options" library="default" rotate="0" style="--rotate-angle: 0deg;"></wa-icon>
      </wa-button>
      <wa-dropdown-item value="convert" @click="${() => this.convertManagerRef.value?.convertSingle(<Track>this._selectedItem)}">Convert</wa-dropdown-item>
      <wa-dropdown-item value="play" @click="${() => this.playManagerRef.value?.playSingle(<Track>this._selectedItem)}">Play</wa-dropdown-item>
    </wa-dropdown>
    <wa-tooltip for="options" placement="bottom" distance="2" without-arrow id="wa-tooltip-4JMAo0Oz3lCxM3wujKNlc">Options</wa-tooltip>
  </div>
  <div class="wa-flank wa-gap-3xl" style="--content-percentage: 40%; margin-bottom: var(--wa-space-m);">
    <div class="wa-frame" style="max-inline-size: 40ch">
      <sonary-cover-dialog
        .thumbUrl="${this._selectedItem?.cover?.["320"]}"
        .fullUrl="${this._selectedItem?.cover?.["640"]}"
        .altText="${this._selectedItem?.title}"></sonary-cover-dialog>
    </div>
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
      </div>
      <div id="play-controls" class="wa-split wa-gap-xl">
        <div class="wa-cluster wa-gap-xl">
          <wa-button class="play-button-single" appearance="plain" slot="trigger" size="s" variant="neutral" @click="${() => this.playManagerRef.value?.playSingle(<Track>this._selectedItem)}">
            <wa-icon name="circle-play" label="Play track" role="img" aria-label="Play track" library="default" rotate="0" style="--rotate-angle: 0deg;"></wa-icon>
          </wa-button>
        </div>
      </div>
    </div>
  </div>
  <div class="wa-flank wa-gap-3xl" style="--content-percentage: 40%">
    <div class="wa-split:column wa-align-items-start">
      <div class="wa-stack" style="margin-block: auto">
        ${this._selectedItem?.lyrics &&
            html`<pre>${this._selectedItem?.lyrics}</pre>`}
      </div>
    </div>
  </div>

  <sonary-convert-manager ${ref(this.convertManagerRef)}></sonary-convert-manager>
  <sonary-play-manager ${ref(this.playManagerRef)}></sonary-play-manager>
</div>
`
    }
}

declare global {
    interface HTMLElementTagNameMap {
        'sonary-tracks-view': TracksView
    }
}
