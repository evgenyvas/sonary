export interface Artist {
    id: number,
    name: string,
}

export interface Album {
    id: number,
    artist: string,
    artist_id: number,
    title: string,
    year: number,
    tracks: Track[],
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
    number: number,
    duration: number,
    lyrics: string,
    like: boolean,
}

export const EventProgressUpdate = "PROGRESS_UPDATE"
export const EventError = "ERROR"
export const EventFinished = "FINISHED"

type EventMap = {
    EventProgressUpdate: {
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
