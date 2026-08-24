import SonaryLitElement from '@/base'
import { html } from 'lit'
import { customElement } from 'lit/decorators.js'
import { setProgressIndeterminate, playTrack } from '@/store'
import type { Track } from '@/types'

@customElement('sonary-play-manager')
export class PlayManager extends SonaryLitElement {
    public playSingle(track: Track) {
        if (track?.id) {
            this._sendPlay([track.id])
        }
    }

    public playBatch(tracks: Track[]) {
        if (!tracks || tracks.length === 0) return
        const trackIds = tracks.map(track => track.id)
        this._sendPlay(trackIds)
    }

    private _sendPlay(trackIds: number[]) {
        this.store.dispatch(setProgressIndeterminate(true))
        this.store.dispatch(playTrack(trackIds)).then(() => {
            this.store.dispatch(setProgressIndeterminate(false))
        }).catch(() => {
            this.store.dispatch(setProgressIndeterminate(false))
        })
    }

    // the service component does not have its own user interface
    render() {
        return html``
    }
}

declare global {
    interface HTMLElementTagNameMap {
        'sonary-play-manager': PlayManager
    }
}
