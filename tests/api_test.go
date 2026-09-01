package api_test

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sonary/internal/api"
	"sonary/internal/database"
	test "sonary/utils/testing"
	"strings"
	"testing"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	if err := database.RegisterSQLiteFunctions(); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open(
		"sqlite",
		"file::memory:?mode=memory&cache=shared",
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Close()
	})
	if err := database.InitSchema(db); err != nil {
		t.Fatal(err)
	}
	return db
}

func newAPI(t *testing.T) (*api.API, *sql.DB) {
	t.Helper()
	db := newTestDB(t)
	return &api.API{
		ReadDB:  db,
		WriteDB: db,
	}, db
}

func seedDatabase(t *testing.T, db *sql.DB) {
	t.Helper()

	_, err := db.Exec(`
		INSERT INTO artists (id, name)
		VALUES
			(1, 'Test Artist'),
			(2, 'Тест Артист')
	`)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(`
		INSERT INTO artist_relations (artist_id, related_artist_id)
		VALUES
			(1, 2)
	`)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(`
		INSERT INTO albums (id, artist_id, title, year)
		VALUES
			(1, 1, 'Test Album', 2024),
			(2, 2, 'Тест Альбом', 1996)
	`)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(`
		INSERT INTO directories (id, path)
		VALUES (1, '/music/test')
	`)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(`
		INSERT INTO tracks (
			id,
			album_id,
			directory_id,
			artist_id,
			path,
			file_type,
			title,
			year,
			genre,
			track_number,
			duration,
			pregap_duration,
			lyrics,
			cue_file,
			cue_offset,
			is_like
		)
		VALUES (
			1,
			1,
			1,
			1,
			'/music/test/song.flac',
			'FLAC',
			'Test Song',
			2024,
			'test genre',
			1,
			262253333333,
			0,
			'',
			'',
			0,
			0
		), (
			2,
			2,
			1,
			2,
			'/music/test/песня.flac',
			'FLAC',
			'Тест Песня',
			1996,
			'тест жанр',
			2,
			354906666666,
			0,
			'',
			'',
			0,
			1
		)
	`)
	if err != nil {
		t.Fatal(err)
	}
}

func getTracks(t *testing.T, testAPI *api.API, url string) api.APITrackList {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, url, nil)
	rec := httptest.NewRecorder()
	testAPI.GetTracks(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body: %s", rec.Code, rec.Body.String())
	}
	test.Equals(t, "application/json", rec.Header().Get("Content-Type"))
	var response api.APITrackList
	test.Ok(t, json.NewDecoder(rec.Body).Decode(&response))
	test.Equals(t, "ok", response.Message)
	return response
}

func TestApiGetTracks(t *testing.T) {
	testAPI, db := newAPI(t)
	seedDatabase(t, db)

	tests := []struct {
		name  string
		url   string
		check func(t *testing.T, response api.APITrackList)
	}{
		{
			name: "default",
			url:  "/api/v1/tracks?mode=ALL",
			check: func(t *testing.T, response api.APITrackList) {
				test.Equals(t, 2, len(response.Items))

				test.Equals(t, 1, response.Items[0].ID)
				test.Equals(t, "Test Song", response.Items[0].Title)
				test.Equals(t, "Test Artist", response.Items[0].Artist)
				test.Equals(t, "Test Album", response.Items[0].Album)

				test.Equals(t, 2, response.Items[1].ID)
				test.Equals(t, "Тест Песня", response.Items[1].Title)
				test.Equals(t, "Тест Артист", response.Items[1].Artist)
				test.Equals(t, "Тест Альбом", response.Items[1].Album)
			},
		},
		{
			name: "limit",
			url:  "/api/v1/tracks?mode=ALL&limit=1",
			check: func(t *testing.T, response api.APITrackList) {
				test.Equals(t, 1, len(response.Items))
				test.Equals(t, 1, response.Items[0].ID)
			},
		},
		{
			name: "search",
			url:  "/api/v1/tracks?mode=ALL&searchQuery=Test",
			check: func(t *testing.T, response api.APITrackList) {
				test.Equals(t, 1, len(response.Items))
				test.Equals(t, 1, response.Items[0].ID)
			},
		},
		{
			name: "search cyrillic",
			url:  "/api/v1/tracks?mode=ALL&searchQuery=песня",
			check: func(t *testing.T, response api.APITrackList) {
				test.Equals(t, 1, len(response.Items))
				test.Equals(t, 2, response.Items[0].ID)
			},
		},
		{
			name: "artist",
			url:  "/api/v1/tracks?mode=ALL&artistId=1",
			check: func(t *testing.T, response api.APITrackList) {
				test.Equals(t, 1, len(response.Items))
				test.Equals(t, 1, response.Items[0].ID)
			},
		},
		{
			name: "favorites",
			url:  "/api/v1/tracks?mode=FAVORITES",
			check: func(t *testing.T, response api.APITrackList) {
				test.Equals(t, 1, len(response.Items))
				test.Equals(t, 2, response.Items[0].ID)
				test.Equals(t, true, response.Items[0].IsLike)
			},
		},
		{
			name: "search not found",
			url:  "/api/v1/tracks?mode=ALL&searchQuery=Unknown",
			check: func(t *testing.T, response api.APITrackList) {
				test.Equals(t, 0, len(response.Items))
			},
		},
		{
			name: "page",
			url:  "/api/v1/tracks?mode=ALL&limit=1&page=2",
			check: func(t *testing.T, response api.APITrackList) {
				test.Equals(t, 1, len(response.Items))
				test.Equals(t, 2, response.Items[0].ID)
				test.Equals(t, false, response.HasNext)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := getTracks(t, testAPI, tt.url)
			tt.check(t, response)
		})
	}
}

func TestApiGetTracksBadRequest(t *testing.T) {
	testAPI, _ := newAPI(t)
	seedDatabase(t, testAPI.ReadDB)

	tests := []struct {
		name string
		url  string
	}{
		{
			name: "invalid mode",
			url:  "/api/v1/tracks?mode=INVALID",
		},
		{
			name: "invalid limit",
			url:  "/api/v1/tracks?limit=abc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			rec := httptest.NewRecorder()

			testAPI.GetTracks(rec, req)

			test.Equals(t, http.StatusBadRequest, rec.Code)
		})
	}
}

func TestApiGetTrack(t *testing.T) {
	testAPI, db := newAPI(t)
	seedDatabase(t, db)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/tracks/{id}", testAPI.GetTrack)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tracks/1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body: %s", rec.Code, rec.Body.String())
	}
	test.Equals(t, "application/json", rec.Header().Get("Content-Type"))

	var response api.APITrackSingle
	test.Ok(t, json.NewDecoder(rec.Body).Decode(&response))
	test.Equals(t, "ok", response.Message)
	test.Equals(t, 1, response.ID)
	test.Equals(t, "Test Song", response.Title)
	test.Equals(t, "Test Artist", response.Artist)
	test.Equals(t, "Test Album", response.Album)
}

func TestApiGetTrackInvalidID(t *testing.T) {
	testAPI, db := newAPI(t)
	seedDatabase(t, db)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tracks/abc", nil)
	req.SetPathValue("id", "abc")

	rec := httptest.NewRecorder()
	testAPI.GetTrack(rec, req)

	test.Equals(t, http.StatusBadRequest, rec.Code)
}

func TestApiUpdateTrack(t *testing.T) {
	testAPI, db := newAPI(t)
	seedDatabase(t, db)

	tests := []struct {
		name  string
		id    string
		body  string
		like  bool
		check func(t *testing.T, response api.APITrackSingle)
	}{
		{
			name: "like",
			id:   "1",
			body: `{"like":true}`,
			like: true,
			check: func(t *testing.T, response api.APITrackSingle) {
				test.Equals(t, 1, response.ID)
				test.Equals(t, true, response.IsLike)
			},
		},
		{
			name: "unlike",
			id:   "2",
			body: `{"like":false}`,
			like: false,
			check: func(t *testing.T, response api.APITrackSingle) {
				test.Equals(t, 2, response.ID)
				test.Equals(t, false, response.IsLike)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(
				http.MethodPatch, "/api/v1/tracks/"+tt.id, strings.NewReader(tt.body),
			)

			req.SetPathValue("id", tt.id)
			rec := httptest.NewRecorder()
			testAPI.UpdateTrack(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected status 200, got %d, body: %s", rec.Code, rec.Body.String())
			}

			test.Equals(t, "application/json", rec.Header().Get("Content-Type"))

			var response api.APITrackSingle
			test.Ok(t, json.NewDecoder(rec.Body).Decode(&response))

			test.Equals(t, http.StatusOK, response.Status)
			test.Equals(t, "ok", response.Message)

			tt.check(t, response)

			var isLike bool
			err := db.QueryRow("SELECT is_like FROM tracks WHERE id = ?", tt.id).Scan(&isLike)

			test.Ok(t, err)
			test.Equals(t, tt.like, isLike)
		})
	}
}

func TestApiUpdateTrackInvalidJSON(t *testing.T) {
	testAPI, db := newAPI(t)
	seedDatabase(t, db)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/tracks/1", strings.NewReader(`{"like":`))

	req.SetPathValue("id", "1")
	rec := httptest.NewRecorder()
	testAPI.UpdateTrack(rec, req)

	test.Equals(t, http.StatusBadRequest, rec.Code)
}

func TestApiUpdateTrackInvalidID(t *testing.T) {
	testAPI, db := newAPI(t)
	seedDatabase(t, db)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/tracks/abc", strings.NewReader(`{"like":true}`))

	req.SetPathValue("id", "abc")
	rec := httptest.NewRecorder()
	testAPI.UpdateTrack(rec, req)

	test.Equals(t, http.StatusBadRequest, rec.Code)
}

func TestApiGetArtists(t *testing.T) {
	testAPI, db := newAPI(t)
	seedDatabase(t, db)

	tests := []struct {
		name  string
		url   string
		check func(t *testing.T, response api.APIArtistList)
	}{
		{
			name: "default",
			url:  "/api/v1/artists?mode=ALL",
			check: func(t *testing.T, response api.APIArtistList) {
				test.Equals(t, 2, len(response.Items))
				test.Equals(t, 1, response.Items[0].ID)
				test.Equals(t, "Test Artist", response.Items[0].Name)
				test.Equals(t, 2, response.Items[1].ID)
				test.Equals(t, "Тест Артист", response.Items[1].Name)
			},
		},
		{
			name: "random",
			url:  "/api/v1/artists?mode=RANDOM",
			check: func(t *testing.T, response api.APIArtistList) {
				test.Equals(t, 2, len(response.Items))
			},
		},
		{
			name: "limit",
			url:  "/api/v1/artists?mode=ALL&limit=1",
			check: func(t *testing.T, response api.APIArtistList) {
				test.Equals(t, 1, len(response.Items))
				test.Equals(t, 1, response.Items[0].ID)
				test.Equals(t, true, response.HasNext)
			},
		},
		{
			name: "page",
			url:  "/api/v1/artists?mode=ALL&limit=1&page=2",
			check: func(t *testing.T, response api.APIArtistList) {
				test.Equals(t, 1, len(response.Items))
				test.Equals(t, 2, response.Items[0].ID)
				test.Equals(t, false, response.HasNext)
			},
		},
		{
			name: "search",
			url:  "/api/v1/artists?mode=ALL&searchQuery=Test",
			check: func(t *testing.T, response api.APIArtistList) {
				test.Equals(t, 1, len(response.Items))
				test.Equals(t, 1, response.Items[0].ID)
			},
		},
		{
			name: "search cyrillic",
			url:  "/api/v1/artists?mode=ALL&searchQuery=тест",
			check: func(t *testing.T, response api.APIArtistList) {
				test.Equals(t, 1, len(response.Items))
				test.Equals(t, 2, response.Items[0].ID)
				test.Equals(t, "Тест Артист", response.Items[0].Name)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			rec := httptest.NewRecorder()

			testAPI.GetArtists(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected status 200, got %d, body: %s", rec.Code, rec.Body.String())
			}

			test.Equals(t, "application/json", rec.Header().Get("Content-Type"))

			var response api.APIArtistList
			test.Ok(t, json.NewDecoder(rec.Body).Decode(&response))

			test.Equals(t, http.StatusOK, response.Status)
			test.Equals(t, "ok", response.Message)

			tt.check(t, response)
		})
	}
}

func TestApiGetArtistsBadRequest(t *testing.T) {
	testAPI, db := newAPI(t)
	seedDatabase(t, db)

	tests := []string{
		"/api/v1/artists?mode=INVALID",
		"/api/v1/artists?limit=abc",
	}

	for _, url := range tests {
		t.Run(url, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, url, nil)
			rec := httptest.NewRecorder()

			testAPI.GetArtists(rec, req)

			test.Equals(t, http.StatusBadRequest, rec.Code)
		})
	}
}

func TestApiGetArtist(t *testing.T) {
	testAPI, db := newAPI(t)
	seedDatabase(t, db)

	_, err := db.Exec(`
		INSERT INTO artists (id, name)
		VALUES (3, 'Lonely Artist')
	`)
	test.Ok(t, err)

	tests := []struct {
		name  string
		id    string
		check func(t *testing.T, response api.APIArtistSingle)
	}{
		{
			name: "artist",
			id:   "1",
			check: func(t *testing.T, response api.APIArtistSingle) {
				test.Equals(t, 1, response.ID)
				test.Equals(t, "Test Artist", response.Name)
				test.Equals(t, 1, len(response.RelatedArtists))
				test.Equals(t, 2, response.RelatedArtists[0].ID)
				test.Equals(t, "Тест Артист", response.RelatedArtists[0].Name)
				test.Equals(t, 0, len(response.Images))
			},
		},
		{
			name: "cyrillic artist",
			id:   "2",
			check: func(t *testing.T, response api.APIArtistSingle) {
				test.Equals(t, 2, response.ID)
				test.Equals(t, "Тест Артист", response.Name)
			},
		},
		{
			name: "artist without related artists",
			id:   "3",
			check: func(t *testing.T, response api.APIArtistSingle) {
				test.Equals(t, 3, response.ID)
				test.Equals(t, "Lonely Artist", response.Name)
				test.Equals(t, 0, len(response.RelatedArtists))
			},
		},
		{
			name: "artist with reverse related artist",
			id:   "2",
			check: func(t *testing.T, response api.APIArtistSingle) {
				test.Equals(t, 2, response.ID)
				test.Equals(t, "Тест Артист", response.Name)
				test.Equals(t, 1, len(response.RelatedArtists))
				test.Equals(t, 1, response.RelatedArtists[0].ID)
				test.Equals(t, "Test Artist", response.RelatedArtists[0].Name)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/artists/"+tt.id, nil)
			req.SetPathValue("id", tt.id)
			rec := httptest.NewRecorder()
			testAPI.GetArtist(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected status 200, got %d, body: %s", rec.Code, rec.Body.String())
			}

			test.Equals(t, "application/json", rec.Header().Get("Content-Type"))

			var response api.APIArtistSingle
			test.Ok(t, json.NewDecoder(rec.Body).Decode(&response))

			test.Equals(t, http.StatusOK, response.Status)
			test.Equals(t, "ok", response.Message)

			tt.check(t, response)
		})
	}
}

func TestApiGetArtistInvalidID(t *testing.T) {
	testAPI, db := newAPI(t)
	seedDatabase(t, db)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/artists/abc", nil)
	req.SetPathValue("id", "abc")
	rec := httptest.NewRecorder()
	testAPI.GetArtist(rec, req)

	test.Equals(t, http.StatusBadRequest, rec.Code)
}

func TestApiGetAlbums(t *testing.T) {
	testAPI, db := newAPI(t)
	seedDatabase(t, db)

	_, err := db.Exec(`
		INSERT INTO artists (id, name)
		VALUES (3, 'Another Artist')
	`)
	test.Ok(t, err)

	_, err = db.Exec(`
		INSERT INTO albums (id, artist_id, title, year)
		VALUES (3, 3, 'Another Album', 2020)
	`)
	test.Ok(t, err)

	tests := []struct {
		name  string
		url   string
		check func(t *testing.T, response api.APIAlbumList)
	}{
		{
			name: "default",
			url:  "/api/v1/albums?mode=ALL",
			check: func(t *testing.T, response api.APIAlbumList) {
				test.Equals(t, 3, len(response.Items))
				test.Equals(t, 1, response.Items[0].ID)
				test.Equals(t, "Test Album", response.Items[0].Title)
				test.Equals(t, 3, response.Items[1].ID)
				test.Equals(t, "Another Album", response.Items[1].Title)
				test.Equals(t, 2, response.Items[2].ID)
				test.Equals(t, "Тест Альбом", response.Items[2].Title)
			},
		},
		{
			name: "limit",
			url:  "/api/v1/albums?mode=ALL&limit=1",
			check: func(t *testing.T, response api.APIAlbumList) {
				test.Equals(t, 1, len(response.Items))
				test.Equals(t, true, response.HasNext)
			},
		},
		{
			name: "page",
			url:  "/api/v1/albums?mode=ALL&limit=1&page=2",
			check: func(t *testing.T, response api.APIAlbumList) {
				test.Equals(t, 1, len(response.Items))
				test.Equals(t, 3, response.Items[0].ID)
				test.Equals(t, true, response.HasNext)
			},
		},
		{
			name: "artist",
			url:  "/api/v1/albums?mode=ALL&artistId=2",
			check: func(t *testing.T, response api.APIAlbumList) {
				test.Equals(t, 1, len(response.Items))
				test.Equals(t, 2, response.Items[0].ID)
				test.Equals(t, "Тест Альбом", response.Items[0].Title)
			},
		},
		{
			name: "search",
			url:  "/api/v1/albums?mode=ALL&searchQuery=Test",
			check: func(t *testing.T, response api.APIAlbumList) {
				test.Equals(t, 1, len(response.Items))
				test.Equals(t, 1, response.Items[0].ID)
			},
		},
		{
			name: "search cyrillic",
			url:  "/api/v1/albums?mode=ALL&searchQuery=тест",
			check: func(t *testing.T, response api.APIAlbumList) {
				test.Equals(t, 1, len(response.Items))
				test.Equals(t, 2, response.Items[0].ID)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			rec := httptest.NewRecorder()

			testAPI.GetAlbums(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected status 200, got %d, body: %s", rec.Code, rec.Body.String())
			}

			test.Equals(t, "application/json", rec.Header().Get("Content-Type"))

			var response api.APIAlbumList
			test.Ok(t, json.NewDecoder(rec.Body).Decode(&response))
			test.Equals(t, http.StatusOK, response.Status)
			test.Equals(t, "ok", response.Message)

			tt.check(t, response)
		})
	}
}

func TestApiGetAlbumsBadRequest(t *testing.T) {
	testAPI, db := newAPI(t)
	seedDatabase(t, db)

	tests := []string{
		"/api/v1/albums?mode=INVALID",
		"/api/v1/albums?limit=abc",
	}

	for _, url := range tests {
		t.Run(url, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, url, nil)
			rec := httptest.NewRecorder()

			testAPI.GetAlbums(rec, req)

			test.Equals(t, http.StatusBadRequest, rec.Code)
		})
	}
}

func TestApiGetAlbum(t *testing.T) {
	testAPI, db := newAPI(t)
	seedDatabase(t, db)

	tests := []struct {
		name  string
		id    string
		check func(t *testing.T, response api.APIAlbumSingle)
	}{
		{
			name: "album",
			id:   "1",
			check: func(t *testing.T, response api.APIAlbumSingle) {
				test.Equals(t, 1, response.ID)
				test.Equals(t, "Test Album", response.Title)
				test.Equals(t, 1, response.ArtistID)
				test.Equals(t, "Test Artist", response.Artist)
				test.Equals(t, 1, len(response.Tracks))
				test.Equals(t, 1, response.Tracks[0].ID)
				test.Equals(t, "Test Song", response.Tracks[0].Title)
				test.Equals(t, "Test Artist", response.Tracks[0].Artist)
			},
		},
		{
			name: "cyrillic album",
			id:   "2",
			check: func(t *testing.T, response api.APIAlbumSingle) {
				test.Equals(t, 2, response.ID)
				test.Equals(t, "Тест Альбом", response.Title)
				test.Equals(t, "Тест Артист", response.Artist)
				test.Equals(t, 1, len(response.Tracks))
				test.Equals(t, 2, response.Tracks[0].ID)
				test.Equals(t, "Тест Песня", response.Tracks[0].Title)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/albums/"+tt.id, nil)
			req.SetPathValue("id", tt.id)
			rec := httptest.NewRecorder()

			testAPI.GetAlbum(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected status 200, got %d, body: %s", rec.Code, rec.Body.String())
			}

			test.Equals(t, "application/json", rec.Header().Get("Content-Type"))

			var response api.APIAlbumSingle
			test.Ok(t, json.NewDecoder(rec.Body).Decode(&response))
			test.Equals(t, http.StatusOK, response.Status)
			test.Equals(t, "ok", response.Message)

			tt.check(t, response)
		})
	}
}
