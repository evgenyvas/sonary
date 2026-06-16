package lib

import (
//"os"
)

type APIStatus struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
}

type APITrackOrDirectory interface {
	APIType() string
}

type APITrack struct {
	Path   string `json:"path"`
	Format string `json:"format"`
	Date   string `json:"date"`
	Title  string `json:"title"`
	Type   string `json:"type"`
}

func (m APITrack) APIType() string {
	return "track"
}

type APIDirectory struct {
	Path string `json:"path"`
	Name string `json:"name"`
	Type string `json:"type"`
}

func (m APIDirectory) APIType() string {
	return "directory"
}

type APITrackSingle struct {
	APIStatus
	APITrack
}

type APITrackList struct {
	APIStatus
	Items []APITrackOrDirectory `json:"items"`
}

type APITrackPost struct {
	Format  string `json:"format"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

type APIScan struct {
	ID        int    `json:"id"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	Status    string `json:"status"`
	Message   string `json:"message"`
	Result    string `json:"result"`
}

type APIPathPost struct {
	Path string `json:"path"`
}

func ToAPI(n TrackOrDirectory) APITrackOrDirectory {
	switch n := n.(type) {
	case Track:
		return APITrack{
			Path: n.Path,
			Type: n.Type(),
		}
	case Album:
		return APIDirectory{
			Name: n.Title,
			Type: n.Type(),
		}
	default:
		return nil
	}
}
