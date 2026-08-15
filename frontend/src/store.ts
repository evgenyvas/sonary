import { legacy_createStore as createStore, combineReducers, applyMiddleware } from 'redux'
import { thunk } from 'redux-thunk'
import type { Track, Artist, Album, ConvertTrackParams } from '@/types'
import httpClient, { flatten } from '@/utils/request'

export interface TracksQuery {
    mode: fetchTracksMode
    artistId?: number
    albumId?: number
}

export function getTracksKey(query: TracksQuery): string {
    return [
        query.mode,
        query.artistId ?? '-',
        query.albumId ?? '-',
    ].join(':')
}

const initialState = {
    progress: <number>0,
    progressIndeterminate: <boolean>false,
}

const initialStateTrack = {
    currentKey: <string>'',
    items: <Track[]>[],
    hasNext: <boolean>false,
    selectedItem: <Track | null>null,
}

export interface ArtistsQuery {
    mode: fetchArtistsMode
    relatedArtistId?: number
}

export function getArtistsKey(query: ArtistsQuery): string {
    return [
        query.mode,
        query.relatedArtistId ?? '-',
    ].join(':')
}

const initialStateArtist = {
    currentKey: <string>'',
    items: <Artist[]>[],
    hasNext: <boolean>false,
    selectedItem: <Artist | null>null,
}

export interface AlbumsQuery {
    mode: fetchAlbumsMode
    artistId?: number
}

export function getAlbumsKey(query: AlbumsQuery): string {
    return [
        query.mode,
        query.artistId ?? '',
    ].join(':')
}

const initialStateAlbum = {
    currentKey: <string>'',
    items: <Album[]>[],
    hasNext: <boolean>false,
    selectedItem: <Album | null>null,
}

const SET_CURRENT_TRACKS_KEY: string = 'SET_CURRENT_TRACKS_KEY'
const SET_TRACK_ITEMS: string = 'SET_TRACK_ITEMS'
const APPEND_TRACK_ITEMS: string = 'APPEND_TRACK_ITEMS'
const SET_TRACK_ITEM: string = 'SET_TRACK_ITEM'
const SET_TRACK_HAS_NEXT: string = 'SET_TRACK_HAS_NEXT'
const UPDATE_ITEM: string = 'UPDATE_ITEM'
const SET_PROGRESS: string = 'SET_PROGRESS'
const SET_PROGRESS_INDETERMINATE: string = 'SET_PROGRESS_INDETERMINATE'
const DELETE_ITEM: string = 'DELETE_ITEM'

const SET_CURRENT_ARTISTS_KEY: string = 'SET_CURRENT_ARTISTS_KEY'
const SET_ARTIST_ITEMS: string = 'SET_ARTIST_ITEMS'
const APPEND_ARTIST_ITEMS: string = 'APPEND_ARTIST_ITEMS'
const SET_ARTIST_ITEM: string = 'SET_ARTIST_ITEM'
const SET_ARTIST_HAS_NEXT: string = 'SET_ARTIST_HAS_NEXT'

const SET_CURRENT_ALBUMS_KEY: string = 'SET_CURRENT_ALBUMS_KEY'
const SET_ALBUM_ITEMS: string = 'SET_ALBUM_ITEMS'
const APPEND_ALBUM_ITEMS: string = 'APPEND_ALBUM_ITEMS'
const SET_ALBUM_ITEM: string = 'SET_ALBUM_ITEM'
const SET_ALBUM_HAS_NEXT: string = 'SET_ALBUM_HAS_NEXT'

export const setCurrentTracksKey = (payload: string) => {
    return {
        type: SET_CURRENT_TRACKS_KEY,
        payload
    }
}

export const setTrackItems = (payload: Track[]) => {
    return {
        type: SET_TRACK_ITEMS,
        payload
    }
}

export const appendTrackItems = (payload: Track[]) => {
    return {
        type: APPEND_TRACK_ITEMS,
        payload
    }
}

export const setTrackItem = (payload: Track | null) => {
    return {
        type: SET_TRACK_ITEM,
        payload
    }
}

export const setTrackHasNext = (payload: boolean) => {
    return {
        type: SET_TRACK_HAS_NEXT,
        payload
    }
}

export const updateItem = (payload: Track) => {
    return {
        type: UPDATE_ITEM,
        payload
    }
}

export const setProgress = (payload: number) => {
    return {
        type: SET_PROGRESS,
        payload
    }
}

export const setProgressIndeterminate = (payload: boolean) => {
    return {
        type: SET_PROGRESS_INDETERMINATE,
        payload
    }
}

export const deleteItem = (payload: number) => {
    return {
        type: DELETE_ITEM,
        payload
    }
}

export const setCurrentArtistsKey = (payload: string) => {
    return {
        type: SET_CURRENT_ARTISTS_KEY,
        payload
    }
}

export const setArtistItems = (payload: Artist[]) => {
    return {
        type: SET_ARTIST_ITEMS,
        payload
    }
}

export const appendArtistItems = (payload: Artist[]) => {
    return {
        type: APPEND_ARTIST_ITEMS,
        payload
    }
}

export const setArtistItem = (payload: Artist | null) => {
    return {
        type: SET_ARTIST_ITEM,
        payload
    }
}

export const setArtistHasNext = (payload: boolean) => {
    return {
        type: SET_ARTIST_HAS_NEXT,
        payload
    }
}

export const setCurrentAlbumsKey = (payload: string) => {
    return {
        type: SET_CURRENT_ALBUMS_KEY,
        payload
    }
}

export const setAlbumItems = (payload: Album[]) => {
    return {
        type: SET_ALBUM_ITEMS,
        payload
    }
}

export const appendAlbumItems = (payload: Album[]) => {
    return {
        type: APPEND_ALBUM_ITEMS,
        payload
    }
}

export const setAlbumItem = (payload: Album | null) => {
    return {
        type: SET_ALBUM_ITEM,
        payload
    }
}

export const setAlbumHasNext = (payload: boolean) => {
    return {
        type: SET_ALBUM_HAS_NEXT,
        payload
    }
}

const appSlice = (state = initialState, action: any) => {
    switch (action.type) {
        case SET_PROGRESS:
            state.progress = action.payload
            break
        case SET_PROGRESS_INDETERMINATE:
            state.progressIndeterminate = action.payload
            break
    }
    return state
}

const tracksSlice = (state = initialStateTrack, action: any) => {
    switch (action.type) {
        case SET_CURRENT_TRACKS_KEY:
            state.currentKey = action.payload
            break
        case SET_TRACK_ITEMS:
            state.items = action.payload
            break
        case APPEND_TRACK_ITEMS:
            state.items.push(...action.payload)
            break
        case SET_TRACK_ITEM:
            state.selectedItem = action.payload
            break
        case SET_TRACK_HAS_NEXT:
            state.hasNext = action.payload
            break
        case UPDATE_ITEM:
            state.items = state.items.map((el: Track) => (el.id === action.payload.id) ? action.payload : el)
            break
        case DELETE_ITEM:
            state.items = state.items.filter((el: Track) => el.id !== action.payload)
    }
    return state
}

const artistsSlice = (state = initialStateArtist, action: any) => {
    switch (action.type) {
        case SET_CURRENT_ARTISTS_KEY:
            state.currentKey = action.payload
            break
        case SET_ARTIST_ITEMS:
            state.items = action.payload
            break
        case APPEND_ARTIST_ITEMS:
            state.items.push(...action.payload)
            break
        case SET_ARTIST_ITEM:
            state.selectedItem = action.payload
            break
        case SET_ARTIST_HAS_NEXT:
            state.hasNext = action.payload
            break
    }
    return state
}

const albumsSlice = (state = initialStateAlbum, action: any) => {
    switch (action.type) {
        case SET_CURRENT_ALBUMS_KEY:
            state.currentKey = action.payload
            break
        case SET_ALBUM_ITEMS:
            state.items = action.payload
            break
        case APPEND_ALBUM_ITEMS:
            state.items.push(...action.payload)
            break
        case SET_ALBUM_ITEM:
            state.selectedItem = action.payload
            break
        case SET_ALBUM_HAS_NEXT:
            state.hasNext = action.payload
            break
    }
    return state
}

const store = createStore(combineReducers({
    app: appSlice,
    tracks: tracksSlice,
    artists: artistsSlice,
    albums: albumsSlice,
}), applyMiddleware(thunk))

export type RootState = ReturnType<typeof store.getState>
export type AppDispatch = typeof store.dispatch

export enum fetchTracksMode {
    All = "ALL",
    Random = "RANDOM",
    Favorites = "FAVORITES",
    NoAlbum = "NOALBUM", // tracks which artist is not equals to album artist
}

export const fetchTracks = (query: TracksQuery,
    limit: number = 50, page: number | null = null): any => {
    return async (dispatch: AppDispatch) => {
        let params: {
            mode: fetchTracksMode, limit: number, page?: number, artistId?: number
        } = { mode: query.mode, limit }
        if (page) {
            params.page = page
        }
        if (query.artistId) {
            params.artistId = query.artistId
        }
        return httpClient('/v1/tracks', { params: flatten(params) })
            .then(response => {
                dispatch(setTrackHasNext(response.next))
                if (page) {
                    dispatch(appendTrackItems(response.items))
                } else {
                    dispatch(setTrackItems(response.items))
                }
            })
    }
}

export const fetchTrack = (trackId: number): any => {
    return async (dispatch: AppDispatch) => {
        const storeState = store.getState()
        if (storeState.tracks.currentKey) { // try to find in already loaded
            let foundItem: Track | undefined = storeState.tracks.items.find((el: Track) => el.id === trackId)
            if (foundItem) {
                return new Promise((resolve) => {
                    resolve(dispatch(setTrackItem(foundItem)))
                })
            }
        }
        return httpClient('/v1/tracks/' + trackId, {
            method: 'GET',
        })
            .then(response => dispatch(setTrackItem(response)))
    }
}

export interface updateTrackParams {
    like: boolean,
}

export const updateTrack = (trackId: number, params: updateTrackParams): any => {
    return async (dispatch: AppDispatch) => {
        return httpClient('/v1/tracks/' + trackId, {
            method: 'PUT',
            body: params
        })
            .then(response => {
                dispatch(updateItem(response))
                dispatch(setTrackItem(response))
            })
    }
}

export enum fetchArtistsMode {
    All = "ALL",
    Random = "RANDOM",
}

export const fetchArtists = (query: ArtistsQuery,
    limit: number = 50, page: number | null = null): any => {
    return async (dispatch: AppDispatch) => {
        let params: {
            mode: fetchArtistsMode, limit: number, page?: number
        } = { mode: query.mode, limit }
        if (page) {
            params.page = page
        }
        return httpClient('/v1/artists', { params: params })
            .then(response => {
                dispatch(setArtistHasNext(response.next))
                if (page) {
                    dispatch(appendArtistItems(response.items))
                } else {
                    dispatch(setArtistItems(response.items))
                }
            })
    }
}

export const fetchArtist = (artistId: number): any => {
    return async (dispatch: AppDispatch) => {
        return httpClient('/v1/artists/' + artistId, {
            method: 'GET',
        })
            .then(response => {
                dispatch(setArtistItem(response))
                dispatch(setArtistItems(response.related))
                dispatch(setArtistHasNext(false))
                dispatch(setCurrentArtistsKey(getArtistsKey({
                    mode: fetchArtistsMode.All,
                    relatedArtistId: artistId,
                })))
            })
    }
}

export enum fetchAlbumsMode {
    All = "ALL",
    Random = "RANDOM",
}

export const fetchAlbums = (query: AlbumsQuery,
    limit: number = 50, page: number | null = null): any => {
    return async (dispatch: AppDispatch) => {
        let params: {
            mode: fetchAlbumsMode, limit: number, page?: number, artistId?: number
        } = { mode: query.mode, limit }
        if (page) {
            params.page = page
        }
        if (query.artistId) {
            params.artistId = query.artistId
        }
        return httpClient('/v1/albums', { params: flatten(params) })
            .then(response => {
                dispatch(setAlbumHasNext(response.next))
                if (page) {
                    dispatch(appendAlbumItems(response.items))
                } else {
                    dispatch(setAlbumItems(response.items))
                }
            })
    }
}

export const fetchAlbum = (albumId: number): any => {
    return async (dispatch: AppDispatch) => {
        return httpClient('/v1/albums/' + albumId, {
            method: 'GET',
        })
            .then(response => {
                dispatch(setAlbumItem(response))
                dispatch(setTrackItems(response.tracks))
                dispatch(setTrackHasNext(false))
                dispatch(setCurrentTracksKey(getTracksKey({
                    mode: fetchTracksMode.All,
                    albumId: albumId,
                })))
            })
    }
}

var getFilename = (filename: string, contentDisposition: string): string => {
    // Search for filename="name.ext" or filename=name.ext
    const utf8Match = contentDisposition.match(/filename\*=UTF-8''([^;\n]*)/i)
    if (utf8Match && utf8Match[1]) {
        filename = decodeURIComponent(utf8Match[1]) // Декодируем %XX обратно в нормальный текст
    } else {
        const filenameMatch = contentDisposition.match(/filename="([^"]+)"/i)
        if (filenameMatch && filenameMatch[1]) {
            filename = filenameMatch[1].replace(/['"]/g, '')
        }
    }
    return filename
}

export const convertTrack = (trackIds: number[], params: ConvertTrackParams): any => {
    return async () => {
        params.track_ids = trackIds
        try {
            const response = await httpClient.raw<Blob, 'blob'>('/v1/convert/start', {
                method: 'POST',
                body: params,
                responseType: 'blob'
            })
            const blob = response._data

            if (!blob) throw new Error('Response data is empty')

            const contentType = response.headers.get('Content-Type')

            if (contentType && contentType.includes('audio/mpeg')) {
                const contentDisposition = response.headers.get('Content-Disposition')
                let filename = 'track.mp3'

                if (contentDisposition) {
                    filename = getFilename(filename, contentDisposition)
                }

                const url = window.URL.createObjectURL(blob)
                const link = document.createElement('a')
                link.href = url
                link.setAttribute('download', filename)
                document.body.appendChild(link)
                link.click()
                link.remove()
                window.URL.revokeObjectURL(url)

                return { directDownload: true }
            }

            const text = await blob.text()
            const jsonResult = JSON.parse(text)

            return { directDownload: false, job_id: jsonResult.job_id }
        } catch (error) {
            console.error('Error during convert start:', error)
            throw error
        }
    }
}

export const downloadConvert = (jobId: number): any => {
    return async () => {
        try {
            const response = await httpClient.raw<Blob, 'blob'>('/v1/convert/download/' + jobId, {
                method: 'POST',
                responseType: 'blob'
            })

            const blob = response._data
            if (!blob) throw new Error('File not received')

            // get filename from response header
            const contentDisposition = response.headers.get('Content-Disposition')

            let filename = ''
            if (contentDisposition) {
                filename = getFilename(filename, contentDisposition)
            }
            if (!filename) throw new Error('File name cannot be determined')

            // start download
            const url = window.URL.createObjectURL(blob)
            const link = document.createElement('a')
            link.href = url
            link.setAttribute('download', filename)

            document.body.appendChild(link)
            link.click()

            link.remove()
            window.URL.revokeObjectURL(url)
        } catch (error) {
            console.error('Error while downloading convert result:', error)
        }
    }
}

export const playTrack = (trackIds: number[]): any => {
    return async () => {
        return httpClient('/v1/play', {
            method: 'POST',
            body: { track_ids: trackIds }
        })
    }
}

export default store
