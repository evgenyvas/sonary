import SonaryLitElement from '@/base'
import { html } from 'lit'
import { customElement, state } from 'lit/decorators.js'
import { createRef, ref } from 'lit/directives/ref.js'
import { repeat } from 'lit/directives/repeat.js'
import { classMap } from 'lit/directives/class-map.js'
import { onMessage } from '@/modules/websocket/websocket'
import { notify } from '@/utils/notifier'
import { setProgressIndeterminate, convertTrack, downloadConvert } from '@/store'
import {
    type Track, type TrackConvert, type ConvertTrackParams,
    EventConvertProgressUpdate, EventConvertTrackProgressUpdate,
    ConvertStatusProcessing, ConvertStatusCompleted, ConvertStatusFailed
} from '@/types'
import '@awesome.me/webawesome/dist/components/dialog/dialog.js'
import '@awesome.me/webawesome/dist/components/progress-bar/progress-bar.js'
import '@awesome.me/webawesome/dist/components/button/button.js'
import '@awesome.me/webawesome/dist/components/icon/icon.js'

@customElement('sonary-convert-manager')
export class ConvertManager extends SonaryLitElement {
    @state() private _convertProgress = 0
    @state() private _convertTracksProgress: { [key: number]: number } = {}
    @state() private _convertTracks: TrackConvert[] = []
    @state() private _convertJobId = 0
    @state() private _pregapTrack: Track | null = null

    private pregapDialogRef = createRef<HTMLElement>()
    private convertDialogRef = createRef<HTMLElement>()
    private _unsubscribeMessage: (() => void) | null = null

    connectedCallback() {
        super.connectedCallback()
        this._initWebSocket()
    }

    disconnectedCallback() {
        if (this._unsubscribeMessage) this._unsubscribeMessage()
        super.disconnectedCallback()
    }

    private _initWebSocket() {
        this._unsubscribeMessage = onMessage((msg) => {
            const eventMsg = JSON.parse(msg)
            if (eventMsg.type === EventConvertProgressUpdate) {
                this._convertProgress = eventMsg.progress
                if (eventMsg.total === eventMsg.processed) {
                    notify('Convert finished successfully', 'success')
                    this.store.dispatch(downloadConvert(this._convertJobId))
                }
            } else if (eventMsg.type === EventConvertTrackProgressUpdate) {
                if ([ConvertStatusProcessing, ConvertStatusCompleted].includes(eventMsg.status)) {
                    this._convertTracksProgress = {
                        ...this._convertTracksProgress,
                        [eventMsg.track_id]: eventMsg.progress
                    }
                    if (eventMsg.status === ConvertStatusCompleted) {
                        notify(`Track ${eventMsg.track_title} converted successfully`, 'success')
                    }
                } else if (eventMsg.status === ConvertStatusFailed) {
                    notify(eventMsg.error, 'danger')
                }
            }
        })
    }

    public convertSingle(track: Track) {
        if (track.pregap && track.pregap_duration > 0) {
            this._pregapTrack = track
            const dialog: any = this.pregapDialogRef.value
            if (dialog) dialog.open = true
        } else {
            this._executeConvert([track], false)
        }
    }

    public convertBatch(tracks: Track[]) {
        this._executeConvert(tracks, false)
    }

    private _handlePregapChoice(includePregap: boolean) {
        const dialog: any = this.pregapDialogRef.value
        if (dialog) dialog.open = false

        if (this._pregapTrack) {
            this._executeConvert([this._pregapTrack], includePregap)
        }
    }

    private _executeConvert(tracks: Track[], includePregap: boolean) {
        this._convertTracks = tracks.map(t => ({ id: t.id, title: t.title }))
        this._convertTracksProgress = {}
        tracks.forEach(t => { this._convertTracksProgress[t.id] = 0 })

        this._convertProgress = 0
        this._convertJobId = 0

        this.store.dispatch(setProgressIndeterminate(true))

        const trackIds = tracks.map(t => t.id)
        this.store.dispatch(convertTrack(trackIds, <ConvertTrackParams>{
            format: 'mp3',
            mode: 'cbr',
            quality: '320',
            include_pregap: includePregap
        })).then((response: any) => {
            if (!response.directDownload) {
                const dialog: any = this.convertDialogRef.value
                if (dialog) dialog.open = true
                this._convertJobId = response.job_id
            }
            this.store.dispatch(setProgressIndeterminate(false))
        })
    }

    private _formatPregapDuration(pregapDuration: number): string {
        if (!pregapDuration) return '00:00'
        const minutes = Math.floor(pregapDuration / 60)
        const seconds = pregapDuration % 60
        return `${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')}`
    }

    render() {
        return html`
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
                <wa-progress-bar data-key="${index}" .value="${this._convertTracksProgress[item.id] || 0}">
                    ${this._convertTracksProgress[item.id] || 0}%
                </wa-progress-bar>
              `)}
            </div>
            ${this._convertProgress >= 100 ? html`
              <wa-button slot="footer" variant="brand" size="s" data-dialog="close">Close</wa-button>
            ` : ''}
          </wa-dialog>
        `
    }
}

declare global {
    interface HTMLElementTagNameMap {
        'sonary-convert-manager': ConvertManager
    }
}
