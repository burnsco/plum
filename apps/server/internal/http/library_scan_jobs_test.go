package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"plum/internal/db"
)

func TestNewLibraryScanManager_StoresLifecycleContext(t *testing.T) {
	cases := []struct {
		name string
		ctx  context.Context
	}{
		{"background", context.Background()},
		{"todo", context.TODO()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m := NewLibraryScanManager(tc.ctx, nil, nil, nil, "")
			if m.lifecycleCtx != tc.ctx {
				t.Fatalf("lifecycleCtx = %v want %v", m.lifecycleCtx, tc.ctx)
			}
		})
	}
}

func TestLibraryScanManager_RunPassesLifecycleContextToDiscovery(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	var recorded context.Context
	old := scanLibraryDiscovery
	scanLibraryDiscovery = func(ctx context.Context, dbConn *sql.DB, root, mediaType string, libraryID int, options db.ScanOptions) (db.ScanDelta, error) {
		recorded = ctx
		return db.ScanDelta{}, errors.New("stub stop")
	}
	t.Cleanup(func() { scanLibraryDiscovery = old })

	lifecycle := context.WithValue(context.Background(), libraryScanTestCtxKey{}, "lifecycle")
	m := NewLibraryScanManager(lifecycle, dbConn, nil, nil, "")
	st := libraryScanStatus{LibraryID: 1, Phase: libraryScanPhaseScanning}
	m.run(1, st, db.LibraryTypeMovie, "/movies")

	if recorded == nil {
		t.Fatal("expected discovery to receive context")
	}
	if recorded != lifecycle {
		t.Fatalf("discovery ctx not lifecycle: %v vs %v", recorded, lifecycle)
	}
}

type libraryScanTestCtxKey struct{}
