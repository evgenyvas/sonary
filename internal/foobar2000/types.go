package foobar2000

type Playlist struct {
	ID        string `json:"id"`
	Index     int    `json:"index"`
	Title     string `json:"title"`
	IsCurrent bool   `json:"isCurrent"`
}

type GetPlaylistsResponse struct {
	Playlists []Playlist `json:"playlists"`
}

type AddPlaylistItemsRequest struct {
	Replace bool     `json:"replace,omitempty"`
	Play    bool     `json:"play,omitempty"`
	Async   bool     `json:"async,omitempty"`
	Items   []string `json:"items"`
}

type PlayItem struct {
	Path        string
	IsCue       bool
	TrackNumber int
}

type playlistKey struct {
	Path        string
	TrackNumber int
}

type PlaylistItem struct {
	Columns []string `json:"columns"`
}

type PlaylistItems struct {
	Items      []PlaylistItem `json:"items"`
	Offset     int            `json:"offset"`
	TotalCount int            `json:"totalCount"`
}

type GetPlaylistItemsResponse struct {
	PlaylistItems PlaylistItems `json:"playlistItems"`
}

type RemovePlaylistItemsRequest struct {
	Items []int `json:"items"`
}

type MovePlaylistItemsRequest struct {
	Items       []int `json:"items"`
	TargetIndex int   `json:"targetIndex"`
}

type PlaylistChanges struct {
	Remove  []int
	Current []int // Current[userIndex] = index in foobar
}
