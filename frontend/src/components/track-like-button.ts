import SonaryLitElement from '@/base'
import { html } from 'lit'
import { customElement, property, state } from 'lit/decorators.js'
import { classMap } from 'lit/directives/class-map.js'
import {
    updateTrack, setProgressIndeterminate, deleteItem, fetchTracksMode, type updateTrackParams
} from '@/store'
import { notify } from '@/utils/notifier'
import '@awesome.me/webawesome/dist/components/button/button.js'
import '@awesome.me/webawesome/dist/components/icon/icon.js'
import '@awesome.me/webawesome/dist/components/tooltip/tooltip.js'

@customElement('sonary-track-like-button')
export class TrackLikeButton extends SonaryLitElement {
    @property({ type: Number }) trackId!: number
    @property({ type: Boolean }) isLiked = false
    @property({ type: String }) listMode = ''

    @state() private _isLoading = false

    private _toggleLike(e: Event) {
        e.preventDefault()
        if (this._isLoading) return

        this._isLoading = true
        this.store.dispatch(setProgressIndeterminate(true))

        const targetValue = !this.isLiked

        this.store.dispatch(updateTrack(this.trackId, <updateTrackParams>{
            like: targetValue
        })).then(() => {
            this._isLoading = false
            this.store.dispatch(setProgressIndeterminate(false))

            if (targetValue) {
                notify('Added to favorites', 'success')
            } else {
                notify('Removed from favorites', 'success')
                if (this.listMode === fetchTracksMode.Favorites) {
                    this.store.dispatch(deleteItem(this.trackId))
                }
            }
        }).catch(() => {
            this._isLoading = false
            this.store.dispatch(setProgressIndeterminate(false))
            notify('Failed to update favorite status', 'danger')
        })
    }

    render() {
        return html`
          <span>
            <wa-button
              @click="${this._toggleLike}"
              id="favorite-${this.trackId}"
              appearance="plain"
              size="s"
              variant="neutral"
              ?disabled="${this._isLoading}"
              aria-labelledby="wa-tooltip-like-${this.trackId}"
            >
              <wa-icon
                name="heart"
                label="Favorite"
                variant="regular"
                role="img"
                aria-label="Favorite"
                library="default"
                class="${classMap({ fav_true: this.isLiked })}"
              ></wa-icon>
            </wa-button>
            <wa-tooltip
              for="favorite-${this.trackId}"
              placement="bottom"
              distance="2"
              without-arrow
              id="wa-tooltip-like-${this.trackId}"
            >
              Favorite
            </wa-tooltip>
          </span>
        `
    }
}

declare global {
    interface HTMLElementTagNameMap {
        'sonary-track-like-button': TrackLikeButton
    }
}
