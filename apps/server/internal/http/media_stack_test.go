package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"plum/internal/arr"
	"plum/internal/db"
	"plum/internal/metadata"
)

func TestMediaStackSettingsHandlerPutPersistsSettings(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	handler := &MediaStackSettingsHandler{DB: dbConn, Arr: arr.NewService()}
	payload := db.MediaStackSettings{
		Radarr: db.MediaStackServiceSettings{
			BaseURL:          "http://radarr.test",
			APIKey:           "radarr-key",
			QualityProfileID: 8,
			RootFolderPath:   "/storage/media/movies",
			SearchOnAdd:      true,
		},
		SonarrTV: db.MediaStackServiceSettings{
			BaseURL:          "http://sonarr.test",
			APIKey:           "sonarr-key",
			QualityProfileID: 4,
			RootFolderPath:   "/storage/media/tv",
			SearchOnAdd:      true,
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPut, "/api/settings/media-stack", bytes.NewReader(raw))
	req = req.WithContext(withUser(req.Context(), &db.User{ID: 1, IsAdmin: true}))
	rec := httptest.NewRecorder()

	handler.Put(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	saved, err := db.GetMediaStackSettings(dbConn)
	if err != nil {
		t.Fatalf("get settings: %v", err)
	}
	if saved.Radarr.BaseURL != "http://radarr.test" || saved.SonarrTV.BaseURL != "http://sonarr.test" {
		t.Fatalf("saved settings = %+v", saved)
	}
}

func TestAddDiscoverTitleReturnsUnavailableWhenServiceNotConfigured(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	handler := &LibraryHandler{DB: dbConn, Arr: arr.NewService()}
	req := httptest.NewRequest(http.MethodPost, "/api/discover/movie/101/add", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("mediaType", "movie")
	rctx.URLParams.Add("tmdbId", "101")
	req = req.WithContext(context.WithValue(withUser(req.Context(), &db.User{ID: 1, IsAdmin: true}), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	handler.AddDiscoverTitle(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAddDiscoverTitleTVRoutesToSonarrUsingTVDBID(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	added := false
	sonarrServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v3/series":
			if added {
				_ = json.NewEncoder(w).Encode([]map[string]any{{
					"id":     22,
					"tmdbId": 202,
					"title":  "Show Match",
				}})
				return
			}
			_ = json.NewEncoder(w).Encode([]any{})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v3/queue":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"page":         1,
				"pageSize":     250,
				"totalRecords": 0,
				"records":      []any{},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v3/series/lookup":
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"title":  "Show Match",
				"tvdbId": 355567,
			}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v3/series":
			added = true
			w.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
	defer sonarrServer.Close()

	if _, err := db.SaveMediaStackSettings(dbConn, db.MediaStackSettings{
		SonarrTV: db.MediaStackServiceSettings{
			BaseURL:          sonarrServer.URL,
			APIKey:           "sonarr-key",
			QualityProfileID: 4,
			RootFolderPath:   "/storage/media/tv",
			SearchOnAdd:      true,
		},
	}); err != nil {
		t.Fatalf("save settings: %v", err)
	}

	handler := &LibraryHandler{
		DB:  dbConn,
		Arr: arr.NewService(),
		Series: &seriesDetailsStub{getSeriesDetails: func(context.Context, int) (*metadata.SeriesDetails, error) {
			return &metadata.SeriesDetails{TVDBID: "355567"}, nil
		}},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/discover/tv/202/add", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("mediaType", "tv")
	rctx.URLParams.Add("tmdbId", "202")
	req = req.WithContext(context.WithValue(withUser(req.Context(), &db.User{ID: 1, IsAdmin: true}), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	handler.AddDiscoverTitle(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var payload metadata.DiscoverAcquisition
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.State != metadata.DiscoverAcquisitionStateAdded {
		t.Fatalf("payload = %+v", payload)
	}
}

func TestGetDownloadsReturnsPartialPayloadWhenOneServiceFails(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	radarrServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v3/movie":
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"id":     11,
				"tmdbId": 303,
				"title":  "Added Movie",
			}})
		case "/api/v3/queue":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"page":         1,
				"pageSize":     250,
				"totalRecords": 1,
				"records": []map[string]any{{
					"id":                    71,
					"movieId":               11,
					"title":                 "Added Movie",
					"trackedDownloadStatus": "downloading",
					"size":                  1000,
					"sizeleft":              250,
				}},
			})
		default:
			t.Fatalf("unexpected radarr path: %s", r.URL.Path)
		}
	}))
	defer radarrServer.Close()

	sonarrServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "sonarr unavailable", http.StatusBadGateway)
	}))
	defer sonarrServer.Close()

	if _, err := db.SaveMediaStackSettings(dbConn, db.MediaStackSettings{
		Radarr: db.MediaStackServiceSettings{
			BaseURL:          radarrServer.URL,
			APIKey:           "radarr-key",
			QualityProfileID: 8,
			RootFolderPath:   "/storage/media/movies",
			SearchOnAdd:      true,
		},
		SonarrTV: db.MediaStackServiceSettings{
			BaseURL:          sonarrServer.URL,
			APIKey:           "sonarr-key",
			QualityProfileID: 4,
			RootFolderPath:   "/storage/media/tv",
			SearchOnAdd:      true,
		},
	}); err != nil {
		t.Fatalf("save settings: %v", err)
	}

	handler := &LibraryHandler{DB: dbConn, Arr: arr.NewService()}
	req := httptest.NewRequest(http.MethodGet, "/api/downloads", nil)
	req = req.WithContext(withUser(req.Context(), &db.User{ID: 1, IsAdmin: true}))
	rec := httptest.NewRecorder()

	handler.GetDownloads(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var payload arr.DownloadsResponse
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Items) != 1 || payload.Items[0].Title != "Added Movie" {
		t.Fatalf("payload = %+v", payload)
	}
}

func TestGetDownloadsReturnsEmptyPayloadWhenOneServiceFailsWithoutActiveDownloads(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	radarrServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v3/movie":
			_ = json.NewEncoder(w).Encode([]map[string]any{})
		case "/api/v3/queue":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"page":         1,
				"pageSize":     250,
				"totalRecords": 0,
				"records":      []map[string]any{},
			})
		default:
			t.Fatalf("unexpected radarr path: %s", r.URL.Path)
		}
	}))
	defer radarrServer.Close()

	sonarrServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "sonarr unavailable", http.StatusBadGateway)
	}))
	defer sonarrServer.Close()

	if _, err := db.SaveMediaStackSettings(dbConn, db.MediaStackSettings{
		Radarr: db.MediaStackServiceSettings{
			BaseURL:          radarrServer.URL,
			APIKey:           "radarr-key",
			QualityProfileID: 8,
			RootFolderPath:   "/storage/media/movies",
			SearchOnAdd:      true,
		},
		SonarrTV: db.MediaStackServiceSettings{
			BaseURL:          sonarrServer.URL,
			APIKey:           "sonarr-key",
			QualityProfileID: 4,
			RootFolderPath:   "/storage/media/tv",
			SearchOnAdd:      true,
		},
	}); err != nil {
		t.Fatalf("save settings: %v", err)
	}

	handler := &LibraryHandler{DB: dbConn, Arr: arr.NewService()}
	req := httptest.NewRequest(http.MethodGet, "/api/downloads", nil)
	req = req.WithContext(withUser(req.Context(), &db.User{ID: 1, IsAdmin: true}))
	rec := httptest.NewRecorder()

	handler.GetDownloads(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var payload arr.DownloadsResponse
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !payload.Configured {
		t.Fatalf("payload = %+v", payload)
	}
	if len(payload.Items) != 0 {
		t.Fatalf("payload = %+v", payload)
	}
}

func TestGetDiscoverAttachesAcquisitionStates(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(
		`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?) RETURNING id`,
		"discover-acq@test.com",
		"hash",
		now,
	).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	var movieLibraryID int
	if err := dbConn.QueryRow(
		`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`,
		userID,
		"Movies",
		db.LibraryTypeMovie,
		"/movies",
		now,
	).Scan(&movieLibraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}
	if _, err := dbConn.Exec(
		`INSERT INTO movies (library_id, title, path, duration, match_status, tmdb_id) VALUES (?, ?, ?, ?, ?, ?)`,
		movieLibraryID,
		"Movie Match",
		"/movies/movie-match.mkv",
		0,
		db.MatchStatusIdentified,
		101,
	); err != nil {
		t.Fatalf("insert movie: %v", err)
	}

	radarrServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v3/movie":
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"id":     11,
				"tmdbId": 303,
				"title":  "Added Movie",
			}})
		case "/api/v3/queue":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"page":         1,
				"pageSize":     250,
				"totalRecords": 1,
				"records": []map[string]any{{
					"id":                    71,
					"movieId":               11,
					"title":                 "Added Movie",
					"trackedDownloadStatus": "downloading",
					"size":                  1000,
					"sizeleft":              250,
				}},
			})
		default:
			t.Fatalf("unexpected radarr path: %s", r.URL.Path)
		}
	}))
	defer radarrServer.Close()

	sonarrServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v3/series":
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"id":     22,
				"tmdbId": 202,
				"title":  "Downloading Show",
			}, {
				"id":     23,
				"tmdbId": 404,
				"title":  "Added Show",
			}})
		case "/api/v3/queue":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"page":         1,
				"pageSize":     250,
				"totalRecords": 1,
				"records": []map[string]any{{
					"id":                    81,
					"seriesId":              22,
					"title":                 "Downloading Show",
					"trackedDownloadStatus": "downloading",
					"size":                  1000,
					"sizeleft":              300,
				}},
			})
		default:
			t.Fatalf("unexpected sonarr path: %s", r.URL.Path)
		}
	}))
	defer sonarrServer.Close()

	if _, err := db.SaveMediaStackSettings(dbConn, db.MediaStackSettings{
		Radarr: db.MediaStackServiceSettings{
			BaseURL:          radarrServer.URL,
			APIKey:           "radarr-key",
			QualityProfileID: 8,
			RootFolderPath:   "/storage/media/movies",
			SearchOnAdd:      true,
		},
		SonarrTV: db.MediaStackServiceSettings{
			BaseURL:          sonarrServer.URL,
			APIKey:           "sonarr-key",
			QualityProfileID: 4,
			RootFolderPath:   "/storage/media/tv",
			SearchOnAdd:      true,
		},
	}); err != nil {
		t.Fatalf("save settings: %v", err)
	}

	handler := &LibraryHandler{
		DB:  dbConn,
		Arr: arr.NewService(),
		Discover: &discoverStub{
			getDiscover: func(context.Context) (*metadata.DiscoverResponse, error) {
				return &metadata.DiscoverResponse{
					Shelves: []metadata.DiscoverShelf{{
						ID:    "trending",
						Title: "Trending",
						Items: []metadata.DiscoverItem{
							{MediaType: metadata.DiscoverMediaTypeMovie, TMDBID: 101, Title: "Movie Match"},
							{MediaType: metadata.DiscoverMediaTypeTV, TMDBID: 202, Title: "Downloading Show"},
							{MediaType: metadata.DiscoverMediaTypeTV, TMDBID: 404, Title: "Added Show"},
							{MediaType: metadata.DiscoverMediaTypeMovie, TMDBID: 505, Title: "Not Added"},
						},
					}},
				}, nil
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/discover", nil)
	req = req.WithContext(withUser(req.Context(), &db.User{ID: userID, IsAdmin: true}))
	rec := httptest.NewRecorder()

	handler.GetDiscover(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var payload metadata.DiscoverResponse
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	items := payload.Shelves[0].Items
	if items[0].Acquisition == nil || items[0].Acquisition.State != metadata.DiscoverAcquisitionStateAvailable {
		t.Fatalf("movie acquisition = %+v", items[0].Acquisition)
	}
	if items[1].Acquisition == nil || items[1].Acquisition.State != metadata.DiscoverAcquisitionStateDownloading {
		t.Fatalf("downloading acquisition = %+v", items[1].Acquisition)
	}
	if items[2].Acquisition == nil || items[2].Acquisition.State != metadata.DiscoverAcquisitionStateAdded {
		t.Fatalf("added acquisition = %+v", items[2].Acquisition)
	}
	if items[3].Acquisition == nil || items[3].Acquisition.State != metadata.DiscoverAcquisitionStateNotAdded || !items[3].Acquisition.CanAdd {
		t.Fatalf("not added acquisition = %+v", items[3].Acquisition)
	}
}
