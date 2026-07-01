import store, { type RootState } from '@/store'
import type { Store } from 'redux'
import { connect } from '@/utils/redux-store-connect-mixin'
import BaseLitElement from '@/utils/base-element'

export default class SonaryLitElement extends connect(store)(BaseLitElement) {
    public store: Store = store
    public storeState: RootState = store.getState()

    // disable shadow dom
    protected createRenderRoot() {
        return this
    }
}
