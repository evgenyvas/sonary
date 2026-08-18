import SonaryLitElement from '@/base'
import { html, nothing } from 'lit'
import { customElement, property, state } from 'lit/decorators.js'
import { ref, createRef } from 'lit/directives/ref.js'
import { repeat } from 'lit/directives/repeat.js'
import store, { fetchAlbum, setProgressIndeterminate } from '@/store'
import type { Track, Album, Image } from '@/types'
import { formatDynamicTime } from '@/utils/func'
import '@awesome.me/webawesome/dist/components/carousel/carousel.js'
import '@awesome.me/webawesome/dist/components/dialog/dialog.js'
import '@/components/tracks-list'
import '@/components/play-manager'

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

    private coversDialogRef = createRef<HTMLElement>()
    private coversCarouselRef = createRef<HTMLElement>()
    private playManagerRef = createRef<any>()

    _viewCover(e: Event) {
        e.preventDefault()
        const dialog: any = this.coversDialogRef.value!
        dialog.open = true
    }

    render() {
        return this.getErrorMessage() || this._isLoading ? '' : html`
<div>
  <div class="wa-flank wa-gap-3xl" style="--content-percentage: 40%; margin-bottom: var(--wa-space-m); margin-top: var(--wa-space-m);">
    ${this._selectedItem?.cover?.["320"] ? html`
    <a href="#" @click="${(e: Event) => this._viewCover(e)}">
      <div class="wa-frame img-cover" style="max-inline-size: 40ch">
        <img src="${this._selectedItem?.cover["320"]}" alt="${this._selectedItem?.title}">
      </div>
    </a>
    ` : nothing}
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
      </div>
      <div id="play-controls" class="wa-split wa-gap-xl">
        <div class="wa-cluster wa-gap-xl">
          <wa-button class="play-button-single" appearance="plain" slot="trigger" size="s" variant="neutral" @click="${() => this.playManagerRef.value?.playBatch(this._selectedItem?.tracks)}">
            <wa-icon name="circle-play" label="Play track" role="img" aria-label="Play track" library="default" rotate="0" style="--rotate-angle: 0deg;"></wa-icon>
          </wa-button>
        </div>
      </div>
    </div>
  </div>
  <div class="wa-flank wa-gap-3xl" style="--content-percentage: 40%">
    <div class="wa-split:column wa-align-items-start">
      <div class="wa-stack" style="margin-block: auto">
        <sonary-tracks-list .baseRoute="${import.meta.env.VITE_BASE_APP_ROUTE}" .albumId=${this._selectedItem?.id}></sonary-tracks-list>
      </div>
    </div>
  </div>

  <wa-dialog
        id="covers-view"
        class="img-dialog"
        light-dismiss="true"
        ${ref(this.coversDialogRef)}
        @wa-after-show=${() => (this.coversCarouselRef.value as any)?.goToSlide(0)}
    >
    ${(this._selectedItem?.images?.length ?? 0) > 1 ? html`
    <div class="dialog-content">
      <wa-carousel pagination navigation loop ${ref(this.coversCarouselRef)}>
        ${repeat(this._selectedItem?.images ?? [], (item: Image) => item.url, (item: Image, index: number) => html`
          <wa-carousel-item>
            <div class="img-container">
              <img src="${item.url}" alt="${item.type}" data-key="${index}">
            </div>
          </wa-carousel-item>
        `)}
      </wa-carousel>
    </div>
    ` : html`
    <div class="wa-text-center"><img src="${this._selectedItem?.cover?.["640"]}" alt="${this._selectedItem?.title}"></div>
    `}
  </wa-dialog>

  <sonary-play-manager ${ref(this.playManagerRef)}></sonary-play-manager>
</div>
`
    }
}

declare global {
    interface HTMLElementTagNameMap {
        'sonary-albums-view': AlbumsView
    }
}
