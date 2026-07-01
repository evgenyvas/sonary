package lib

import (
	"sync"
	"sync/atomic"
)

var (
	instance *ImportContext
	once     sync.Once
)

const EventProgressUpdate = "PROGRESS_UPDATE"

type Progress struct {
	Total     int
	Processed atomic.Int64
}

type ImportContext struct {
	ArtistCache map[string]int
	AlbumCache  map[string]int
	Progress    Progress
}

func GetImportContext(reset bool) *ImportContext {
	once.Do(func() {
		instance = &ImportContext{}
	})
	if reset {
		instance.ArtistCache = map[string]int{}
		instance.AlbumCache = map[string]int{}
		instance.Progress.Total = 0
		instance.Progress.Processed.Store(0)
	}
	return instance
}
