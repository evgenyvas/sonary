// Package foobar2000
package foobar2000

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"sonary/internal/config"
	db "sonary/internal/database"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Player struct {
	cfg    *config.Config
	db     *sql.DB
	client *http.Client
}

var (
	instance *Player
	once     sync.Once
)

func GetPlayer() *Player {
	once.Do(func() {
		instance = &Player{
			cfg: config.GetConfig(),
			db:  db.Reader(),
			client: &http.Client{
				Timeout: 5 * time.Second,
			},
		}
	})
	return instance
}

func (p *Player) Play(trackIDs []int) error {
	items, err := p.buildPlayItems(trackIDs)
	if err != nil {
		return err
	}

	if p.cfg.FoobarAPI {
		if err := p.playBeefweb(items); err != nil {
			log.Printf("Beefweb failed: %v", err)
		} else {
			return nil
		}
	}

	return p.playCLI(items)
}

func (p *Player) buildPlayItems(trackIDs []int) ([]PlayItem, error) {
	items := make([]PlayItem, 0, len(trackIDs))

	for _, id := range trackIDs {

		t, err := db.GetTrack(p.db, id)
		if err != nil {
			log.Printf("Cannot load track %d: %v", id, err)
			continue
		}

		item := PlayItem{
			Path:        p.trackToPath(t),
			IsCue:       t.IsCue,
			TrackNumber: t.TrackNumber,
		}

		items = append(items, item)
	}

	if len(items) == 0 {
		return nil, fmt.Errorf("no playable tracks")
	}

	return items, nil
}

func (p *Player) trackToPath(t *db.Track) string {
	path := t.Path
	if t.IsCue {
		path = t.CueFile
	}
	for _, mapping := range p.cfg.PathMap {
		from := strings.TrimRight(mapping.From, "/\\")
		to := strings.TrimRight(mapping.To, "/\\")
		if path == from {
			return to
		}
		prefix := from + "/"
		if !strings.HasPrefix(path, prefix) {
			continue
		}
		relative := strings.TrimPrefix(path, prefix)
		relative = strings.ReplaceAll(relative, "/", "\\")
		return to + `\` + relative
	}
	return path
}

func (p *Player) playBeefweb(items []PlayItem) error {
	playlistID, err := p.getCurrentPlaylistID()
	if err != nil {
		return err
	}

	if err := p.addItems(playlistID, items); err != nil {
		return err
	}

	playlist, err := p.getPlaylistItems(playlistID)
	if err != nil {
		return err
	}

	changes := p.findPlaylistChanges(items, playlist)

	if len(changes.Remove) > 0 {
		if err := p.removeItems(playlistID, changes.Remove); err != nil {
			return err
		}
		// adjust indexes
		for i := range changes.Current {
			shift := 0
			for _, r := range changes.Remove {
				if r < changes.Current[i] {
					shift++
				}
			}
			changes.Current[i] -= shift
		}
	}

	if err := p.reorderPlaylist(playlistID, changes.Current); err != nil {
		return err
	}

	// after manipulations with playlist - play the first track
	return p.playItem(playlistID, 0)
}

func (p *Player) reorderPlaylist(playlistID string, current []int) error {
	for target := 0; target < len(current); target++ {
		from := current[target]
		if from == target {
			continue
		}
		if err := p.moveItems(playlistID, []int{from}, target); err != nil {
			return err
		}
		// update indexes
		for i := range current {
			switch {
			case current[i] == from:
				current[i] = target

			case from > target:
				if current[i] >= target && current[i] < from {
					current[i]++
				}

			case from < target:
				if current[i] > from && current[i] <= target {
					current[i]--
				}
			}
		}
	}
	return nil
}

func (p *Player) findPlaylistChanges(
	items []PlayItem,
	playlist []PlaylistItem,
) PlaylistChanges {

	wanted := make(map[playlistKey]int, len(items))

	for i, item := range items {
		key := playlistKey{
			Path: item.Path,
		}
		if item.IsCue {
			key.TrackNumber = item.TrackNumber
		}
		wanted[key] = i
	}

	result := PlaylistChanges{
		Current: make([]int, len(items)),
	}

	for playlistIndex, item := range playlist {
		if len(item.Columns) == 0 {
			continue
		}

		key := playlistKey{
			Path: item.Columns[0],
		}
		if len(item.Columns) > 1 {
			n, err := strconv.Atoi(item.Columns[1])
			if err == nil {
				key.TrackNumber = n
			}
		}

		wantedIndex, ok := wanted[key]
		if !ok {
			result.Remove = append(result.Remove, playlistIndex)
			continue
		}

		result.Current[wantedIndex] = playlistIndex
	}

	return result
}

// Fallback via CLI
func (p *Player) playCLI(items []PlayItem) error {

	// tracks could be from the same album in CUE sheet
	// so we clear these items as so CLI opens full CUE album anyway

	uniquePaths := make(map[string]bool)
	var playPaths []string // ordered paths
	for _, item := range items {
		if !uniquePaths[item.Path] {
			uniquePaths[item.Path] = true
			playPaths = append(playPaths, item.Path)
		}
	}

	// send to foobar2000

	// first file via /immediate to clear old playlist
	cmd := exec.Command(p.cfg.FoobarPath, "/immediate", playPaths[0])
	if err := cmd.Start(); err != nil {
		log.Printf("Failed to execution foobar2000 /immediate: %v", err)
		return err
	}

	for _, path := range playPaths[1:] {
		cmd := exec.Command(p.cfg.FoobarPath, "/add", path)
		if err := cmd.Run(); err != nil {
			log.Printf("Cannot add %s to foobar: %v", path, err)
		}
	}

	return nil
}
