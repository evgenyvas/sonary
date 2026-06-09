import TracksLitElement from './tracks-base'
import { css, html } from 'lit'
import { customElement, state } from 'lit/decorators.js'
import { Router } from '@lit-labs/router'
import './tracks-list'
import '@awesome.me/webawesome/dist/components/breadcrumb/breadcrumb.js';
import '@awesome.me/webawesome/dist/components/breadcrumb-item/breadcrumb-item.js';
import type { Track } from '../types'
import type { RootState } from '../store'

@customElement('sonary-tracks')
export class Tracks extends TracksLitElement {
    private _router: Router = new Router(this, [
        {
            path: import.meta.env.VITE_BASE_APP_ROUTE,
            render: () => html`
        <wa-breadcrumb>
          <span slot="separator">/</span>
          <wa-breadcrumb-item>Tracks</wa-breadcrumb-item>
        </wa-breadcrumb>
        <sonary-tracks-list .baseRoute="${import.meta.env.VITE_BASE_APP_ROUTE}"></sonary-tracks-list>
      `
        },
        {
            path: import.meta.env.VITE_BASE_APP_ROUTE + ':path',
            render: ({ path }) => html`
        <wa-breadcrumb>
          <span slot="separator">/</span>
          <wa-breadcrumb-item href="${import.meta.env.VITE_BASE_APP_ROUTE}">Tracks</wa-breadcrumb-item>
          <wa-breadcrumb-item>${this._item?.name}</wa-breadcrumb-item>
        </wa-breadcrumb>
        <sonary-tracks-view .path="${path}" .baseRoute="${import.meta.env.VITE_BASE_APP_ROUTE}"></sonary-tracks-view>
      `
        },
    ])

    @state()
    private _item: Track | null = null

    stateChanged(state: RootState): void {
        super.stateChanged(state)
        this._item = state.tracks.selectedItem
    }

    render() {
        return this.getErrorMessage() || html`
      <div part="base">
        <div>${this._router.outlet()}</div>
      </div>
    `
    }

    static styles = [
        TracksLitElement.styles,
        css`
      :host {
        font-family: var(--wa-font-sans);
        font-size: var(--wa-font-size-large);
      }

      sl-breadcrumb-item::part(base) {
        margin-bottom: var(--wa-spacing-small);
        font-size: var(--wa-font-size-x-large);
      }
    `
    ]
}

declare global {
    interface HTMLElementTagNameMap {
        'sonary-tracks': Tracks
    }
}
