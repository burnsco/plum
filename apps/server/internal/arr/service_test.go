package arr

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"plum/internal/db"
	"plum/internal/metadata"
)

func TestValidateReturnsServiceOptions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v3/rootfolder":
			_ = json.NewEncoder(w).Encode([]RootFolderOption{{Path: "/storage/media/movies"}})
		case "/api/v3/qualityprofile":
			_ = json.NewEncoder(w).Encode([]QualityProfileOption{{ID: 8, Name: "HD Bluray + WEB"}})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	service := NewService()
	result, err := service.Validate(context.Background(), db.MediaStackSettings{
		Radarr: db.MediaStackServiceSettings{
			BaseURL:          server.URL,
			APIKey:           "token",
			QualityProfileID: 8,
			RootFolderPath:   "/storage/media/movies",
			SearchOnAdd:      true,
		},
	})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !result.Radarr.Configured || !result.Radarr.Reachable {
		t.Fatalf("expected radarr to be configured and reachable: %+v", result.Radarr)
	}
	if len(result.Radarr.RootFolders) != 1 || result.Radarr.RootFolders[0].Path != "/storage/media/movies" {
		t.Fatalf("root folders = %+v", result.Radarr.RootFolders)
	}
	if len(result.Radarr.QualityProfiles) != 1 || result.Radarr.QualityProfiles[0].ID != 8 {
		t.Fatalf("quality profiles = %+v", result.Radarr.QualityProfiles)
	}
	if result.SonarrTV.Configured || result.SonarrTV.Reachable {
		t.Fatalf("expected sonarr-tv to remain unconfigured: %+v", result.SonarrTV)
	}
}

func TestLoadSnapshotBuildsCatalogAndDownloads(t *testing.T) {
	radarrServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v3/movie":
			_ = json.NewEncoder(w).Encode([]movieRecord{{ID: 11, TMDBID: 101, Title: "Movie Match"}})
		case "/api/v3/queue":
			_ = json.NewEncoder(w).Encode(queuePage{
				Page:         1,
				PageSize:     queuePageSize,
				TotalRecords: 1,
				Records: []queueRecord{{
					ID:                    77,
					MovieID:               11,
					Title:                 "Movie Match",
					Status:                "queued",
					TrackedDownloadStatus: "downloading",
					Size:                  1000,
					SizeLeft:              250,
					TimeLeft:              "00:05:00",
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
			_ = json.NewEncoder(w).Encode([]seriesRecord{{ID: 22, TMDBID: 202, Title: "Show Match"}})
		case "/api/v3/queue":
			_ = json.NewEncoder(w).Encode(queuePage{
				Page:         1,
				PageSize:     queuePageSize,
				TotalRecords: 1,
				Records: []queueRecord{{
					ID:                    88,
					SeriesID:              22,
					Title:                 "Show Match",
					Status:                "queued",
					TrackedDownloadStatus: "downloading",
					Size:                  2000,
					SizeLeft:              500,
					TimeLeft:              "00:10:00",
				}},
			})
		default:
			t.Fatalf("unexpected sonarr path: %s", r.URL.Path)
		}
	}))
	defer sonarrServer.Close()

	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	service := NewService()
	service.Now = func() time.Time { return now }

	settings := db.MediaStackSettings{
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
	}

	snapshot, err := service.LoadSnapshot(context.Background(), settings)
	if err != nil {
		t.Fatalf("load snapshot: %v", err)
	}
	if len(snapshot.downloads) != 2 {
		t.Fatalf("downloads = %+v", snapshot.downloads)
	}

	movieAcquisition := service.ResolveDiscoverAcquisition(
		metadata.DiscoverMediaTypeMovie,
		101,
		false,
		settings,
		snapshot,
	)
	if movieAcquisition.State != metadata.DiscoverAcquisitionStateDownloading {
		t.Fatalf("movie acquisition = %+v", movieAcquisition)
	}
	showAcquisition := service.ResolveDiscoverAcquisition(
		metadata.DiscoverMediaTypeTV,
		202,
		false,
		settings,
		snapshot,
	)
	if showAcquisition.State != metadata.DiscoverAcquisitionStateDownloading {
		t.Fatalf("show acquisition = %+v", showAcquisition)
	}
	availableAcquisition := service.ResolveDiscoverAcquisition(
		metadata.DiscoverMediaTypeMovie,
		101,
		true,
		settings,
		snapshot,
	)
	if availableAcquisition.State != metadata.DiscoverAcquisitionStateAvailable {
		t.Fatalf("available acquisition = %+v", availableAcquisition)
	}
}

func TestAddMovieUsesConfiguredDefaults(t *testing.T) {
	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v3/movie/lookup/tmdb":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"title":  "Movie Match",
				"tmdbId": 101,
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v3/movie":
			if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
				t.Fatalf("decode add body: %v", err)
			}
			w.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	service := NewService()
	err := service.AddMovie(context.Background(), db.MediaStackServiceSettings{
		BaseURL:          server.URL,
		APIKey:           "radarr-key",
		QualityProfileID: 8,
		RootFolderPath:   "/storage/media/movies",
		SearchOnAdd:      false,
	}, 101)
	if err != nil {
		t.Fatalf("add movie: %v", err)
	}
	if captured["qualityProfileId"] != float64(8) {
		t.Fatalf("qualityProfileId = %#v", captured["qualityProfileId"])
	}
	if captured["rootFolderPath"] != "/storage/media/movies" {
		t.Fatalf("rootFolderPath = %#v", captured["rootFolderPath"])
	}
	addOptions, ok := captured["addOptions"].(map[string]any)
	if !ok || addOptions["searchForMovie"] != true {
		t.Fatalf("addOptions = %#v", captured["addOptions"])
	}
}

func TestAddSeriesUsesConfiguredDefaults(t *testing.T) {
	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v3/series/lookup":
			if got := r.URL.Query().Get("term"); !strings.Contains(got, "tvdb:355567") {
				t.Fatalf("lookup term = %q", got)
			}
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"title":  "Show Match",
				"tvdbId": 355567,
			}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v3/series":
			if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
				t.Fatalf("decode add body: %v", err)
			}
			w.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	service := NewService()
	err := service.AddSeries(context.Background(), db.MediaStackServiceSettings{
		BaseURL:          server.URL,
		APIKey:           "sonarr-key",
		QualityProfileID: 4,
		RootFolderPath:   "/storage/media/tv",
		SearchOnAdd:      false,
	}, "355567")
	if err != nil {
		t.Fatalf("add series: %v", err)
	}
	if captured["qualityProfileId"] != float64(4) {
		t.Fatalf("qualityProfileId = %#v", captured["qualityProfileId"])
	}
	if captured["rootFolderPath"] != "/storage/media/tv" {
		t.Fatalf("rootFolderPath = %#v", captured["rootFolderPath"])
	}
	if captured["seasonFolder"] != true {
		t.Fatalf("seasonFolder = %#v", captured["seasonFolder"])
	}
	addOptions, ok := captured["addOptions"].(map[string]any)
	if !ok ||
		addOptions["monitor"] != "all" ||
		addOptions["searchForMissingEpisodes"] != true ||
		addOptions["searchForCutoffUnmetEpisodes"] != true {
		t.Fatalf("addOptions = %#v", captured["addOptions"])
	}
}
