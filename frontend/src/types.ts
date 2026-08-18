export interface Image {
    url: string,
    type: string,
    order: number,
}

type ImageSizes = { [key: number]: string }

interface RelatedArtist {
    id: number,
    name: string,
}

export interface Artist {
    id: number,
    name: string,
    logo?: ImageSizes,
    images: Image[],
    related: RelatedArtist[],
}

export interface Album {
    id: number,
    artist: string,
    artist_id: number,
    artist_logo?: ImageSizes,
    title: string,
    year: number,
    tracks: Track[],
    cover?: ImageSizes,
    images: Image[],
}

export interface Track {
    id: number,
    type: string,
    title: string,
    artist: string,
    artist_id: number,
    albumArtist: string,
    year: number,
    genre: string,
    album: string,
    album_id: number,
    cover?: ImageSizes,
    number: number,
    duration: number,
    pregap: boolean,
    pregap_duration: number,
    lyrics: string,
    like: boolean,
}

export interface TrackConvert {
    id: number,
    title: string,
}

export const EventImportProgressUpdate = "IMPORT_PROGRESS_UPDATE"
export const EventConvertProgressUpdate = "CONVERT_PROGRESS_UPDATE"
export const EventConvertTrackProgressUpdate = "CONVERT_TRACK_PROGRESS_UPDATE"

export const ConvertStatusProcessing = "PROCESSING"
export const ConvertStatusCompleted = "COMPLETED"
export const ConvertStatusFailed = "FAILED"

export const EventError = "ERROR"
export const EventFinished = "FINISHED"

type EventMap = {
    EventImportProgressUpdate: {
        progress: number
    }

    EventError: {
        message: string
        code: number
    }

    EventFinished: {
        result: string
    }
}

export type EventMsg = {
    [K in keyof EventMap]: {
        type: K
    } & EventMap[K]
}[keyof EventMap]

export interface ConvertTrackParams {
    userId: string,
    track_ids: number[],
    format: string,
    mode: string,
    quality: string,
    include_pregap: boolean,
}
