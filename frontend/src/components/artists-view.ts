import SonaryLitElement from '@/base'
import { html } from 'lit'
import { customElement, property, state } from 'lit/decorators.js'
import store, { type RootState, fetchArtist, setProgressIndeterminate, fetchTracksMode } from '@/store'
import type { Artist } from '@/types'
import { classMap } from 'lit/directives/class-map.js'
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
        }
    }

    render() {
        return this.getErrorMessage() || this._isLoading ? '' : html`
<div>
  <div class="wa-cluster"><br></div>
  <h1 class="wa-heading-4xl">${this._selectedItem?.name}</h1>
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
</div>
`
    }
}

declare global {
    interface HTMLElementTagNameMap {
        'sonary-artists-view': ArtistsView
    }
}
