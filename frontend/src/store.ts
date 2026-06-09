import { legacy_createStore as createStore, combineReducers, applyMiddleware } from 'redux'
import { thunk } from 'redux-thunk'
import type { Track } from './types'
import httpClient from '@/utils/request'

const initialState = {
    isInitList: <boolean>false,
    items: <Track[]>[],
    selectedItem: <Track | null>null,
}

const SET_ITEMS: string = 'SET_ITEMS'
const SET_ITEM: string = 'SET_ITEM'

export const setItems = (payload: Track[]) => {
    return {
        type: SET_ITEMS,
        payload
    }
}

export const setItem = (payload: Track | null) => {
    return {
        type: SET_ITEM,
        payload
    }
}

const tracksSlice = (state = initialState, action: any) => {
    switch (action.type) {
        case SET_ITEMS:
            state.items = action.payload
            break
        case SET_ITEM:
            state.selectedItem = action.payload
            break
    }
    return state
}

const store = createStore(combineReducers({
    tracks: tracksSlice,
}), applyMiddleware(thunk))

export type RootState = ReturnType<typeof store.getState>
export type AppDispatch = typeof store.dispatch

export const fetchTracks = (path: string): any => {
    return async (dispatch: AppDispatch) => {
        return httpClient('/v1/tracks' + path)
            .then(response => {
                dispatch(setItems(response.items))
            })
    }
}

export default store
