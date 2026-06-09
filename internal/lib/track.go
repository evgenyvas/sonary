// Package lib
package lib

import (
	"time"
)

type TrackOrDirectory interface {
	Type() string
}

type Track struct {
	Path        string
	FullContent string
	Content     []byte
	Metadata    MetadataTrack
}

func (m Track) Type() string {
	return "note"
}

type DirectoryTrack struct {
	Path string
	Name string
}

func (m DirectoryTrack) Type() string {
	return "directory"
}

type MetadataTrack struct {
	Format string
	Date   time.Time
	Title  string
}

func ParseMetadata(input string) (meta MetadataTrack, rest []byte) {
	meta = MetadataTrack{
		Format: "",
		Title:  "",
		Date:   time.Now(),
	}

	//t, err := time.Parse("2006-01-02 15:04:05", "")
	//if err != nil {
	//log.Println("Error parsing date:", err)
	//} else {
	//meta.Date = t
	//}

	return meta, rest
}
