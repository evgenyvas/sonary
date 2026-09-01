// Package context
package context

import (
	"sync"
	"sync/atomic"
)

var (
	importCT  *ImportContext
	convertCT *ConvertContext
)

const EventImportProgressUpdate = "IMPORT_PROGRESS_UPDATE"
const EventConvertProgressUpdate = "CONVERT_PROGRESS_UPDATE"
const EventConvertTrackProgressUpdate = "CONVERT_TRACK_PROGRESS_UPDATE"

func init() {
	importCT = &ImportContext{}
	convertCT = &ConvertContext{}
}

type ImportProgress struct {
	Total     int
	Processed atomic.Int64
}

type ImportContext struct {
	ArtistCache map[string]int
	AlbumCache  map[string]int
	Progress    ImportProgress
}

func GetImportContext() *ImportContext {
	return importCT
}

func ResetImportContext() {
	clear(importCT.ArtistCache)
	clear(importCT.AlbumCache)
	importCT.Progress.Total = 0
	importCT.Progress.Processed.Store(0)
}

type ConvertProgress struct {
	Total     int
	Processed atomic.Int64
}

type ConvertContext struct {
	Cache    sync.Map
	Jobs     sync.Map // jobId: FilePath
	Progress ConvertProgress
}

func GetConvertContext() *ConvertContext {
	return convertCT
}

func ResetConvertProgress() {
	convertCT.Jobs.Clear()
	convertCT.Progress.Total = 0
	convertCT.Progress.Processed.Store(0)
}

func ClearConvertCache() {
	// remove temporary files
	//os.Remove(tmpPath)
	convertCT.Cache.Clear()
}
