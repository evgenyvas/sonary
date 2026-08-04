package foobar2000

import (
	"fmt"
)

const playlistFetchAll = 100000

func (p *Player) getPlaylistItems(playlistID string) ([]PlaylistItem, error) {
	var result GetPlaylistItemsResponse

	err := p.get(fmt.Sprintf(
		"/playlists/%s/items/0:%d?columns=%%path%%,%%subsong%%",
		playlistID,
		playlistFetchAll,
	), &result)
	if err != nil {
		return nil, err
	}

	return result.PlaylistItems.Items, nil
}

func (p *Player) getCurrentPlaylistID() (string, error) {
	var result GetPlaylistsResponse
	err := p.get("/playlists", &result)
	if err != nil {
		return "", err
	}

	for _, playlist := range result.Playlists {
		if playlist.IsCurrent {
			return playlist.ID, nil
		}
	}

	return "", fmt.Errorf("current playlist not found")
}

func (p *Player) playItem(playlistID string, index int) error {
	return p.post(fmt.Sprintf("/player/play/%s/%d", playlistID, index), nil)
}

func (p *Player) addItems(playlistID string, items []PlayItem) error {
	req := AddPlaylistItemsRequest{
		Replace: true,
		Play:    false,
	}

	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		if _, ok := seen[item.Path]; ok {
			continue
		}
		seen[item.Path] = struct{}{}
		req.Items = append(req.Items, item.Path)
	}

	return p.post(fmt.Sprintf("/playlists/%s/items/add", playlistID), req)
}

func (p *Player) removeItems(playlistID string, indexes []int) error {
	return p.post(
		fmt.Sprintf("/playlists/%s/items/remove", playlistID),
		RemovePlaylistItemsRequest{
			Items: indexes,
		},
	)
}

func (p *Player) moveItems(playlistID string, items []int, target int) error {
	return p.post(
		fmt.Sprintf("/playlists/%s/items/move", playlistID),
		MovePlaylistItemsRequest{
			Items:       items,
			TargetIndex: target,
		},
	)
}
