import { html } from 'lit'
import { customElement, state } from 'lit/decorators.js'
import { setProgress, type RootState, fetchTracksMode, fetchAlbumsMode } from '@/store'
import { Router } from '@lit-labs/router'
import SonaryLitElement from '@/base'
import '@/assets/style.scss'
import { connect as wsConnect, onMessage } from '@/modules/websocket/websocket'
import { classMap } from 'lit/directives/class-map.js'
import logoImage from '@/assets/logo_icon.png'
import '@awesome.me/webawesome/dist/components/page/page.js'
import '@awesome.me/webawesome/dist/components/button/button.js'
import '@awesome.me/webawesome/dist/components/icon/icon.js'
import '@awesome.me/webawesome/dist/components/input/input.js'
import '@awesome.me/webawesome/dist/components/tooltip/tooltip.js'
import '@awesome.me/webawesome/dist/components/dropdown/dropdown.js'
import '@awesome.me/webawesome/dist/components/dropdown-item/dropdown-item.js'
import '@awesome.me/webawesome/dist/components/progress-bar/progress-bar.js'
import '@awesome.me/webawesome/dist/components/breadcrumb/breadcrumb.js'
import '@awesome.me/webawesome/dist/components/breadcrumb-item/breadcrumb-item.js'
import { EventProgressUpdate } from '@/types'
import '@/components/tracks-list'
import '@/components/tracks-view'
import '@/components/artists-list'
import '@/components/artists-view'
import '@/components/albums-list'
import '@/components/albums-view'

@customElement('sonary-app')
export class SonaryApp extends SonaryLitElement {
    private _router: Router = new Router(this, [
        {
            path: import.meta.env.VITE_BASE_APP_ROUTE,
            render: () => html`
        <wa-breadcrumb>
          <span slot="separator">/</span>
          <wa-breadcrumb-item>Tracks</wa-breadcrumb-item>
        </wa-breadcrumb>
        <sonary-tracks-list .baseRoute="${import.meta.env.VITE_BASE_APP_ROUTE}" .mode=${fetchTracksMode.Random}></sonary-tracks-list>
      `
        },
        {
            path: import.meta.env.VITE_BASE_APP_ROUTE + 'favorites',
            render: () => html`
        <wa-breadcrumb>
          <span slot="separator">/</span>
          <wa-breadcrumb-item>Favorites</wa-breadcrumb-item>
        </wa-breadcrumb>
        <sonary-tracks-list .baseRoute="${import.meta.env.VITE_BASE_APP_ROUTE}" .mode=${fetchTracksMode.Favorites}></sonary-tracks-list>
      `
        },
        {
            path: import.meta.env.VITE_BASE_APP_ROUTE + 'artists',
            render: () => html`
        <wa-breadcrumb>
          <span slot="separator">/</span>
          <wa-breadcrumb-item>Artists</wa-breadcrumb-item>
        </wa-breadcrumb>
        <sonary-artists-list .baseRoute="${import.meta.env.VITE_BASE_APP_ROUTE}"></sonary-artists-list>
      `
        },
        {
            path: import.meta.env.VITE_BASE_APP_ROUTE + 'artists/:id',
            render: ({ id }) => html`
        <wa-breadcrumb>
          <span slot="separator">/</span>
          <wa-breadcrumb-item href="${import.meta.env.VITE_BASE_APP_ROUTE}artists">Artists</wa-breadcrumb-item>
          <wa-breadcrumb-item>View artist</wa-breadcrumb-item>
        </wa-breadcrumb>
        <sonary-artists-view .id="${id}" .baseRoute="${import.meta.env.VITE_BASE_APP_ROUTE}"></sonary-artists-view>
      `
        },
        {
            path: import.meta.env.VITE_BASE_APP_ROUTE + 'albums',
            render: () => html`
        <wa-breadcrumb>
          <span slot="separator">/</span>
          <wa-breadcrumb-item>Albums</wa-breadcrumb-item>
        </wa-breadcrumb>
        <sonary-albums-list .baseRoute="${import.meta.env.VITE_BASE_APP_ROUTE}" .mode=${fetchAlbumsMode.Random}></sonary-albums-list>
      `
        },
        {
            path: import.meta.env.VITE_BASE_APP_ROUTE + 'albums/:id',
            render: ({ id }) => html`
        <wa-breadcrumb>
          <span slot="separator">/</span>
          <wa-breadcrumb-item href="${import.meta.env.VITE_BASE_APP_ROUTE}">Albums</wa-breadcrumb-item>
          <wa-breadcrumb-item>View album</wa-breadcrumb-item>
        </wa-breadcrumb>
        <sonary-albums-view .id="${id}" .baseRoute="${import.meta.env.VITE_BASE_APP_ROUTE}"></sonary-albums-view>
      `
        },
        {
            path: import.meta.env.VITE_BASE_APP_ROUTE + ':id',
            render: ({ id }) => html`
        <wa-breadcrumb>
          <span slot="separator">/</span>
          ${this.keyMode === fetchTracksMode.Random
                    ? html`<wa-breadcrumb-item href="${import.meta.env.VITE_BASE_APP_ROUTE}">Tracks</wa-breadcrumb-item>`
                    : html`<wa-breadcrumb-item href="${import.meta.env.VITE_BASE_APP_ROUTE + 'favorites'}">Favorites</wa-breadcrumb-item>`}
          <wa-breadcrumb-item>View track</wa-breadcrumb-item>
        </wa-breadcrumb>
        <sonary-tracks-view .id="${id}" .baseRoute="${import.meta.env.VITE_BASE_APP_ROUTE}"></sonary-tracks-view>
      `
        },
    ])

    @state()
    private _baseRoute: string = '/'

    @state()
    private _progress: number = 0

    @state()
    private _progressIndeterminate: boolean = false

    @state()
    private _progressVisible: boolean = false

    private get keyMode(): fetchTracksMode {
        return this.storeState.tracks.currentKey.startsWith(fetchTracksMode.Favorites)
            ? fetchTracksMode.Favorites : fetchTracksMode.Random
    }

    connectedCallback() {
        super.connectedCallback()

        this._baseRoute = import.meta.env.VITE_BASE_APP_ROUTE

        wsConnect()

        onMessage((msg) => {
            let eventMsg = JSON.parse(msg)
            if (eventMsg.type === EventProgressUpdate) {
                this.store.dispatch(setProgress(eventMsg.progress))
            }
        })
    }

    stateChanged(state: RootState): void {
        super.stateChanged(state)

        this._progress = state.app.progress
        this._progressIndeterminate = state.app.progressIndeterminate
        this._progressVisible = (state.app.progress > 0 && state.app.progress < 100)
            || state.app.progressIndeterminate
    }

    render() {
        return html`
      <wa-progress-bar id="progress-bar" class=${classMap({
            'wa-visually-hidden': !this._progressVisible
        })} .value="${this._progress}" ?indeterminate="${this._progressIndeterminate}">Scanning library ${this._progress}%</wa-progress-bar>
    <wa-page view="desktop" navigation-placement="start" disable-navigation-toggle="">
      <div id="main-content" slot="skip-to-content-target"></div>
      <div id="callout-toast-container" style="position: fixed; bottom: 20px; right: 20px; z-index: 9999; display: flex; flex-direction: column; gap: 10px; max-width: 350px; width: 100%;"></div>
      <header slot="header">
          <a href="${this._baseRoute}" class="wa-link-plain">
        <div class="wa-cluster">
            <wa-button data-toggle-nav="" appearance="plain" size="s" variant="neutral">
              <wa-icon name="bars" label="Menu" role="img" aria-label="Menu" library="default" rotate="0" style="--rotate-angle: 0deg;"></wa-icon>
            </wa-button>
            <div class="wa-frame wa-border-radius-l" style="max-inline-size: 5ch; float: left;">
              <img src="${logoImage}" alt="Sonary" style="width: 100%">
            </div>
            <span class="wa-heading-l">Sonary</span>
        </div>
          </a>
        <wa-input id="search-header" placeholder="Search" class="wa-desktop-only" style="max-inline-size: 100%" type="text" size="m" appearance="outlined">
          <wa-icon slot="start" name="magnifying-glass" aria-hidden="true" library="default" rotate="0" style="--rotate-angle: 0deg;"></wa-icon>
        </wa-input>
        <div class="wa-cluster">
          <wa-dropdown class="color-scheme-selector" title="Toggle color scheme" size="m" placement="bottom-start" @wa-select="${this._changeColorScheme}">
            <wa-button slot="trigger" id="color-scheme-selector-trigger" appearance="plain" pill="" variant="neutral" size="m" aria-labelledby="color-scheme-tooltip">
              <wa-icon name="sun" variant="regular" class="only-light" label="Select Color Scheme" role="img" aria-label="Select Color Scheme" library="default" rotate="0" style="--rotate-angle: 0deg;"></wa-icon>
              <wa-icon name="moon" variant="regular" class="only-dark" label="Select Color Scheme" role="img" aria-label="Select Color Scheme" library="default" rotate="0" style="--rotate-angle: 0deg;"></wa-icon>
            </wa-button>
            <wa-dropdown-item value="light" variant="default" size="m" type="normal" tabindex="0" aria-disabled="false" role="menuitem">
              <wa-icon slot="icon" name="sun" variant="regular" aria-hidden="true" library="default" rotate="0" style="--rotate-angle: 0deg;"></wa-icon>
              Light
            </wa-dropdown-item>
            <wa-dropdown-item value="dark" variant="default" size="m" type="normal" tabindex="-1" aria-disabled="false" role="menuitem">
              <wa-icon slot="icon" name="moon" variant="regular" aria-hidden="true" library="default" rotate="0" style="--rotate-angle: 0deg;"></wa-icon>
              Dark
            </wa-dropdown-item>
            <wa-divider role="separator" aria-orientation="horizontal" orientation="horizontal"></wa-divider>
            <wa-dropdown-item value="auto" variant="default" size="m" type="normal" tabindex="-1" aria-disabled="false" role="menuitem">
              <wa-icon slot="icon" name="circle-half-stroke" variant="regular" aria-hidden="true" library="default" rotate="0" style="--rotate-angle: 0deg;"></wa-icon>
              System
            </wa-dropdown-item>
          </wa-dropdown>
        </div>
      </header>
      <div slot="navigation-header" class="wa-split wa-mobile-only">
        <wa-input id="search-nav-drawer" placeholder="Search" style="max-inline-size: 100%" type="text" size="m" appearance="outlined">
          <wa-icon slot="start" name="magnifying-glass" aria-hidden="true" library="default" rotate="0" style="--rotate-angle: 0deg;"></wa-icon>
        </wa-input>
      </div>
      <nav slot="navigation" style="padding-top: 0;">
        <ul class="wa-stack wa-gap-0">
          <li>
            <a href="${this._baseRoute}favorites" class="wa-flank" data-drawer="close">
              <wa-icon name="heart" aria-hidden="true" library="default" rotate="0" style="--rotate-angle: 0deg;"></wa-icon>
              <span>Favorites</span>
            </a>
          </li>
          <li>
            <a href="${this._baseRoute}artists" class="wa-flank" data-drawer="close">
              <wa-icon name="microphone-lines" aria-hidden="true" library="default" rotate="0" style="--rotate-angle: 0deg;"></wa-icon>
              <span>Artists</span>
            </a>
          </li>
          <li>
            <a href="${this._baseRoute}albums" class="wa-flank" data-drawer="close">
              <wa-icon name="layer-group" aria-hidden="true" library="default" rotate="0" style="--rotate-angle: 0deg;"></wa-icon>
              <span>Albums</span>
            </a>
          </li>
        </ul>
      </nav>
      <main>
        <div class="wa-stack wa-gap-3xl">
          <div part="base">
            <div>${this._router.outlet()}</div>
          </div>
        </div>
      </main>
    </wa-page>
    `
    }

    private _changeColorScheme(e: any) {
        let selectedValue = e.detail.item.value
        if (selectedValue === 'auto') {
            selectedValue = getPreferredSchemeMedia() ? 'dark' : 'light'
        }
        applyScheme(selectedValue === 'dark')
        localStorage.setItem('wa-color-scheme', selectedValue)
    }
}

declare global {
    interface HTMLElementTagNameMap {
        'sonary-app': SonaryApp
    }
}

function applyScheme(dark: boolean) {
    document.documentElement.classList.toggle('wa-dark', dark)
}

function getPreferredSchemeMedia() {
    return window.matchMedia('(prefers-color-scheme: dark)').matches
}

function getPreferredScheme() {
    const savedMode = localStorage.getItem('wa-color-scheme');
    if (savedMode !== null) return savedMode === 'dark'
    return getPreferredSchemeMedia()
}

applyScheme(getPreferredScheme())

window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', event => {
    const savedMode = localStorage.getItem('wa-color-scheme')
    if (!savedMode) {
        applyScheme(event.matches)
    }
})
