import SonaryLitElement from '@/base'
import { html, nothing } from 'lit'
import { customElement, property, state } from 'lit/decorators.js'
import { ref, createRef, type Ref } from 'lit/directives/ref.js'
import { repeat } from 'lit/directives/repeat.js'
import store, { type RootState, fetchArtist, setProgressIndeterminate, fetchTracksMode } from '@/store'
import type { Artist, Image } from '@/types'
import { classMap } from 'lit/directives/class-map.js'
import '@awesome.me/webawesome/dist/components/carousel/carousel.js'
import '@/components/albums-list'
import '@/components/tracks-list'

@customElement('sonary-artists-view')
export class ArtistsView extends SonaryLitElement {
    @property({ type: Number, attribute: 'id' })
    artistId: number | null = null

    @state()
    private _selectedItem: Artist | null = null

    @state()
    private _isLoading: boolean = false

    @property({ type: String, attribute: 'base-route' })
    baseRoute: string = '/'

    @state()
    private _hasAlbums = false

    @state()
    private _hasTracks = false

    connectedCallback() {
        super.connectedCallback()
    }

    _loadItems() {
        this._isLoading = true
        this.store.dispatch(setProgressIndeterminate(true))
        store.dispatch(fetchArtist(<number>this.artistId)).then(() => {
            this._isLoading = false
            this.store.dispatch(setProgressIndeterminate(false))
            this._selectedItem = this.storeState.artists.selectedItem
        })
    }

    // store state changed
    stateChanged(state: RootState): void {
        super.stateChanged(state)
    }

    willUpdate(changedProperties: Map<string, any>) {
        if (changedProperties.has('artistId')) {
            this._hasAlbums = false
            this._hasTracks = false
            this._loadItems()
        }
    }

    private get hasRelated(): boolean {
        return this.storeState.artists.selectedItem?.related != undefined &&
            this.storeState.artists.selectedItem?.related.length > 0
    }

    viewCoversDialogRef: Ref<HTMLInputElement> = createRef()
    coversCarouselRef: Ref<HTMLInputElement> = createRef()

    _viewCovers() {
        const dialog: any = this.viewCoversDialogRef.value!
        dialog.open = true
    }

    render() {
        return this.getErrorMessage() || this._isLoading ? '' : html`
<div>
  <div class="wa-cluster"><br></div>
  <h1 class="wa-heading-4xl">${this._selectedItem?.name}</h1>
  ${this._selectedItem?.logo?.["320"] ? ((this._selectedItem?.images?.length ?? 0) > 1 ? html`
    <a href="#" @click="${() => this._viewCovers()}">
      <div class="img-logo" style="margin-bottom: var(--wa-space-m);">
        <img src="${this._selectedItem?.logo["320"]}" alt="${this._selectedItem?.name}">
      </div>
    </a>` : html`<div class="img-logo" style="margin-bottom: var(--wa-space-m);">
      <img src="${this._selectedItem?.logo["320"]}" alt="${this._selectedItem?.name}">
    </div>`) : nothing}
  <div class="${classMap({ 'wa-visually-hidden': !this._hasAlbums })}">
    <b>Albums</b>
    <sonary-albums-list .baseRoute="${this.baseRoute}" .artistId=${this.artistId} limit="0"
        @albums-empty="${() => this._hasAlbums = false}"
        @albums-loaded="${() => this._hasAlbums = true}"></sonary-albums-list>
  </div>
  <div class="wa-cluster"><br></div>
  <div class="${classMap({ 'wa-visually-hidden': !this._hasTracks })}">
    <b>Tracks</b>
    <sonary-tracks-list .baseRoute="${this.baseRoute}" .mode=${fetchTracksMode.NoAlbum} .artistId=${this.artistId} limit="0"
        @tracks-empty="${() => this._hasTracks = false}"
        @tracks-loaded="${() => this._hasTracks = true}"></sonary-tracks-list>
  </div>
  <div class="${classMap({ 'wa-visually-hidden': !this.hasRelated })}">
    <b>See also</b>
    <sonary-artists-list .baseRoute="${import.meta.env.VITE_BASE_APP_ROUTE}" .relatedArtistId=${this._selectedItem?.id}></sonary-artists-list>
  </div>

  <wa-dialog
        id="covers-view"
        class="img-dialog"
        light-dismiss="true"
        ${ref(this.viewCoversDialogRef)}
        @wa-after-show=${() => (this.coversCarouselRef.value as any)?.goToSlide(0)}
    >
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
  </wa-dialog>
</div>
`
    }
}

declare global {
    interface HTMLElementTagNameMap {
        'sonary-artists-view': ArtistsView
    }
}
