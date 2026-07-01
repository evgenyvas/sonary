import SonaryLitElement from '@/base'
import { html } from 'lit'
import { customElement, property, state } from 'lit/decorators.js'
import store, { fetchArtist, setProgressIndeterminate, fetchTracksMode } from '@/store'
import type { Artist } from '@/types'
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

  render() {
    return this.getErrorMessage() || this._isLoading ? '' : html`
<div>
  <div class="wa-cluster"><br></div>
  <h1 class="wa-heading-4xl">${this._selectedItem?.name}</h1>
  <sonary-albums-list .baseRoute="${this.baseRoute}" .artistId=${this.artistId} limit="0"></sonary-albums-list>
  <sonary-tracks-list .baseRoute="${this.baseRoute}" .mode=${fetchTracksMode.NoAlbum} .artistId=${this.artistId} limit="0"></sonary-tracks-list>
</div>
`
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'sonary-artists-view': ArtistsView
  }
}
