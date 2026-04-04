package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"plum/internal/db"
	"plum/internal/metadata"
	"plum/internal/ws"
)

const (
	libraryScanProgressFlushInterval = 500 * time.Millisecond
	libraryScanProgressFlushEvery    = 25
	libraryScanActivityFlushInterval = 200 * time.Millisecond
	libraryScanDebounceWindow        = 300 * time.Millisecond
	libraryScanActivityMaxRecent     = 20
)

var scanLibraryDiscovery = db.ScanLibraryDiscovery
var enrichLibraryTasks = db.EnrichLibraryTasks

const (
	libraryScanPhaseIdle      = "idle"
	libraryScanPhaseQueued    = "queued"
	libraryScanPhaseScanning  = "scanning"
	libraryScanPhaseCompleted = "completed"
	libraryScanPhaseFailed    = "failed"

	libraryEnrichmentPhaseIdle    = "idle"
	libraryEnrichmentPhaseQueued  = "queued"
	libraryEnrichmentPhaseRunning = "running"

	libraryIdentifyPhaseIdle        = "idle"
	libraryIdentifyPhaseQueued      = "queued"
	libraryIdentifyPhaseIdentifying = "identifying"
	libraryIdentifyPhaseCompleted   = "completed"
	libraryIdentifyPhaseFailed      = "failed"
)

type libraryScanStatus struct {
	LibraryID         int                  `json:"libraryId"`
	Phase             string               `json:"phase"`
	EnrichmentPhase   string               `json:"enrichmentPhase"`
	Enriching         bool                 `json:"enriching"`
	IdentifyPhase     string               `json:"identifyPhase"`
	Identified        int                  `json:"identified"`
	IdentifyFailed    int                  `json:"identifyFailed"`
	Processed         int                  `json:"processed"`
	Added             int                  `json:"added"`
	Updated           int                  `json:"updated"`
	Removed           int                  `json:"removed"`
	Unmatched         int                  `json:"unmatched"`
	Skipped           int                  `json:"skipped"`
	IdentifyRequested bool                 `json:"identifyRequested"`
	QueuedAt          string               `json:"queuedAt,omitempty"`
	EstimatedItems    int                  `json:"estimatedItems"`
	QueuePosition     int                  `json:"queuePosition"`
	Error             string               `json:"error,omitempty"`
	RetryCount        int                  `json:"retryCount"`
	MaxRetries        int                  `json:"maxRetries"`
	NextRetryAt       string               `json:"nextRetryAt,omitempty"`
	LastError         string               `json:"lastError,omitempty"`
	NextScheduledAt   string               `json:"nextScheduledAt,omitempty"`
	StartedAt         string               `json:"startedAt,omitempty"`
	FinishedAt        string               `json:"finishedAt,omitempty"`
	Activity          *libraryScanActivity `json:"activity,omitempty"`
}

type libraryScanActivity struct {
	Stage   string                     `json:"stage"`
	Current *libraryScanActivityEntry  `json:"current,omitempty"`
	Recent  []libraryScanActivityEntry `json:"recent"`
}

type libraryScanActivityEntry struct {
	Phase        string `json:"phase"`
	Target       string `json:"target"`
	RelativePath string `json:"relativePath"`
	At           string `json:"at"`
}

type LibraryScanManager struct {
	db   *sql.DB
	hub  *ws.Hub
	meta metadata.Identifier

	mu             sync.Mutex
	jobs           map[int]libraryScanStatus
	types          map[int]string
	paths          map[int]string
	owners         map[int]int
	subpaths       map[int][]string
	activities     map[int]libraryScanActivity
	reruns         map[int]scanStartRequest
	debounceReady  map[int]bool
	debounceTimers map[int]*time.Timer
	activityTimers map[int]*time.Timer
	retryTimers    map[int]*time.Timer
	enrichCancels  map[int]context.CancelFunc
	watcherStops   map[int]context.CancelFunc
	schedulerStops map[int]context.CancelFunc
	enrichSem      chan struct{}
	identifySem    chan struct{}
	handler        *LibraryHandler
	lastFlushed    map[int]libraryScanFlushState
	lastActivityAt map[int]time.Time
	activeScanID   int
}

type libraryScanFlushState struct {
	at        time.Time
	processed int
}

type queuedLibrary struct {
	id       int
	queuedAt string
}

type scanStartRequest struct {
	identify bool
	subpaths []string
}

type recoveredEnrichmentRun struct {
	libraryID         int
	path              string
	libraryType       string
	identifyRequested bool
}

func normalizeEnrichmentPhase(phase string, enriching bool) string {
	switch phase {
	case libraryEnrichmentPhaseIdle, libraryEnrichmentPhaseQueued, libraryEnrichmentPhaseRunning:
		return phase
	default:
		if enriching {
			return libraryEnrichmentPhaseRunning
		}
		return libraryEnrichmentPhaseIdle
	}
}

func cloneLibraryScanActivity(activity libraryScanActivity) *libraryScanActivity {
	cloned := activity
	if activity.Current != nil {
		current := *activity.Current
		cloned.Current = &current
	}
	cloned.Recent = append([]libraryScanActivityEntry(nil), activity.Recent...)
	if cloned.Recent == nil {
		cloned.Recent = []libraryScanActivityEntry{}
	}
	return &cloned
}

func NewLibraryScanManager(sqlDB *sql.DB, meta metadata.Identifier, hub *ws.Hub) *LibraryScanManager {
	return &LibraryScanManager{
		db:             sqlDB,
		hub:            hub,
		meta:           meta,
		jobs:           make(map[int]libraryScanStatus),
		types:          make(map[int]string),
		paths:          make(map[int]string),
		owners:         make(map[int]int),
		subpaths:       make(map[int][]string),
		activities:     make(map[int]libraryScanActivity),
		reruns:         make(map[int]scanStartRequest),
		debounceReady:  make(map[int]bool),
		debounceTimers: make(map[int]*time.Timer),
		activityTimers: make(map[int]*time.Timer),
		retryTimers:    make(map[int]*time.Timer),
		enrichCancels:  make(map[int]context.CancelFunc),
		watcherStops:   make(map[int]context.CancelFunc),
		schedulerStops: make(map[int]context.CancelFunc),
		enrichSem: make(chan struct{}, 1),
		// More than one library can identify at a time so a long TV pass does not
		// block movie libraries (each run still has its own rate limiter).
		identifySem: make(chan struct{}, 2),
		lastFlushed:    make(map[int]libraryScanFlushState),
		lastActivityAt: make(map[int]time.Time),
	}
}

func (m *LibraryScanManager) AttachHandler(handler *LibraryHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handler = handler
}

func (m *LibraryScanManager) Recover() error {
	statuses, err := db.ListLibraryJobStatuses(m.db)
	if err != nil {
		return err
	}
	automationConfigs, err := loadLibraryAutomationConfigs(m.db)
	if err != nil {
		return err
	}

	var (
		identifiesToResume  []int
		enrichmentsToResume []recoveredEnrichmentRun
	)

	m.mu.Lock()
	for _, persisted := range statuses {
		status := persistedLibraryStatusToRuntime(persisted)
		m.jobs[status.LibraryID] = status
		m.types[status.LibraryID] = persisted.Type
		m.paths[status.LibraryID] = persisted.Path
		delete(m.lastFlushed, status.LibraryID)

		switch {
		case status.Phase == libraryScanPhaseQueued || status.Phase == libraryScanPhaseScanning:
			status.Phase = libraryScanPhaseQueued
			status.EnrichmentPhase = libraryEnrichmentPhaseIdle
			status.Enriching = false
			if status.IdentifyPhase == libraryIdentifyPhaseQueued || status.IdentifyPhase == libraryIdentifyPhaseIdentifying {
				status.IdentifyPhase = libraryIdentifyPhaseIdle
				status.Identified = 0
				status.IdentifyFailed = 0
			}
			m.jobs[status.LibraryID] = status
			m.setActivityStageLocked(status.LibraryID, "queued", true, true)
			m.debounceReady[status.LibraryID] = true
		case status.Phase == libraryScanPhaseCompleted &&
			(status.Enriching ||
				status.EnrichmentPhase == libraryEnrichmentPhaseQueued ||
				status.EnrichmentPhase == libraryEnrichmentPhaseRunning):
			status.EnrichmentPhase = libraryEnrichmentPhaseIdle
			status.Enriching = false
			m.jobs[status.LibraryID] = status
			enrichmentsToResume = append(enrichmentsToResume, recoveredEnrichmentRun{
				libraryID:         status.LibraryID,
				path:              persisted.Path,
				libraryType:       persisted.Type,
				identifyRequested: status.IdentifyRequested,
			})
		case status.Enriching || status.EnrichmentPhase == libraryEnrichmentPhaseQueued || status.EnrichmentPhase == libraryEnrichmentPhaseRunning:
			status.Enriching = false
			status.EnrichmentPhase = libraryEnrichmentPhaseIdle
			status.Phase = libraryScanPhaseQueued
			status.FinishedAt = ""
			m.jobs[status.LibraryID] = status
			m.setActivityStageLocked(status.LibraryID, "queued", true, true)
			m.debounceReady[status.LibraryID] = true
		}
		if status.IdentifyRequested &&
			(status.IdentifyPhase == libraryIdentifyPhaseQueued || status.IdentifyPhase == libraryIdentifyPhaseIdentifying) &&
			status.Phase == libraryScanPhaseCompleted {
			identifiesToResume = append(identifiesToResume, status.LibraryID)
		}
		if status.NextRetryAt != "" && status.RetryCount < status.MaxRetries {
			retryAt, parseErr := time.Parse(time.RFC3339, status.NextRetryAt)
			if parseErr == nil {
				delay := time.Until(retryAt)
				if delay < 0 {
					delay = 0
				}
				retryIdentify := status.Phase == libraryScanPhaseCompleted && status.IdentifyPhase == libraryIdentifyPhaseFailed
				if timer, ok := m.retryTimers[status.LibraryID]; ok {
					timer.Stop()
				}
				libraryID := status.LibraryID
				m.retryTimers[libraryID] = time.AfterFunc(delay, func() {
					m.mu.Lock()
					delete(m.retryTimers, libraryID)
					current := m.jobs[libraryID]
					current.NextRetryAt = ""
					current.Error = ""
					m.jobs[libraryID] = current
					path := m.paths[libraryID]
					libraryType := m.types[libraryID]
					subpaths := append([]string(nil), m.subpaths[libraryID]...)
					m.mu.Unlock()
					m.flushStatus(libraryID, true)
					if retryIdentify {
						m.startIdentify(libraryID)
						return
					}
					m.start(libraryID, path, libraryType, current.IdentifyRequested, subpaths)
				})
			}
		}
	}
	m.mu.Unlock()
	m.flushAllStatuses(true)
	m.scheduleNext()

	for _, libraryID := range identifiesToResume {
		m.startIdentify(libraryID)
	}
	for _, enrichment := range enrichmentsToResume {
		m.resumeRecoveredEnrichment(
			enrichment.libraryID,
			enrichment.path,
			enrichment.libraryType,
			enrichment.identifyRequested,
		)
	}
	for _, cfg := range automationConfigs {
		m.ConfigureLibraryAutomation(cfg.LibraryID, cfg.Path, cfg.Type, cfg.WatcherEnabled, cfg.WatcherMode, cfg.ScanIntervalMinutes)
	}

	return nil
}

func (m *LibraryScanManager) resumeRecoveredEnrichment(
	libraryID int,
	path string,
	libraryType string,
	identifyRequested bool,
) {
	tasks, err := db.ListLibraryEnrichmentTasks(context.Background(), m.db, libraryID, libraryType, identifyRequested)
	if err != nil {
		log.Printf("recover enrichment library=%d type=%s: %v", libraryID, libraryType, err)
		return
	}
	if len(tasks) == 0 {
		m.mu.Lock()
		status, ok := m.jobs[libraryID]
		if ok {
			status.EnrichmentPhase = libraryEnrichmentPhaseIdle
			status.Enriching = false
			m.jobs[libraryID] = status
			m.finalizeActivityLocked(libraryID, status)
		}
		m.mu.Unlock()
		m.flushStatus(libraryID, true)
		return
	}
	m.startEnrichment(libraryID, libraryType, path, nil, tasks, identifyRequested)
}

func (m *LibraryScanManager) start(libraryID int, path, libraryType string, identify bool, subpaths []string) libraryScanStatus {
	if !m.canIdentifyLibrary(libraryType) {
		identify = false
	}
	normalizedSubpaths, _ := db.NormalizeScanSubpaths(subpaths)

	m.mu.Lock()
	if cancel, ok := m.enrichCancels[libraryID]; ok {
		cancel()
		delete(m.enrichCancels, libraryID)
	}

	status, ok := m.jobs[libraryID]
	if ok && status.Phase == libraryScanPhaseScanning {
		request := mergeScanStartRequest(m.reruns[libraryID], scanStartRequest{identify: identify, subpaths: normalizedSubpaths})
		m.reruns[libraryID] = request
		status.IdentifyRequested = status.IdentifyRequested || identify
		m.jobs[libraryID] = status
		result := m.statusLocked(libraryID)
		m.mu.Unlock()
		m.flushAllStatuses(true)
		return result
	}
	if ok && status.Phase == libraryScanPhaseQueued {
		status.IdentifyRequested = status.IdentifyRequested || identify
		m.clearRetryStateLocked(libraryID, &status)
		m.jobs[libraryID] = status
		m.types[libraryID] = libraryType
		m.paths[libraryID] = path
		m.subpaths[libraryID] = mergeScanSubpaths(m.subpaths[libraryID], normalizedSubpaths)
		m.setActivityStageLocked(libraryID, "queued", true, true)
		m.debounceReady[libraryID] = false
		m.scheduleDebounceLocked(libraryID)
		result := m.statusLocked(libraryID)
		m.mu.Unlock()
		m.flushAllStatuses(true)
		return result
	}
	if ok {
		m.clearRetryStateLocked(libraryID, &status)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	status = libraryScanStatus{
		LibraryID:         libraryID,
		Phase:             libraryScanPhaseQueued,
		EnrichmentPhase:   libraryEnrichmentPhaseIdle,
		IdentifyPhase:     libraryIdentifyPhaseIdle,
		IdentifyRequested: identify,
		QueuedAt:          now,
		MaxRetries:        3,
		LastError:         "",
		StartedAt:         now,
	}
	m.jobs[libraryID] = status
	m.types[libraryID] = libraryType
	m.paths[libraryID] = path
	m.subpaths[libraryID] = normalizedSubpaths
	m.setActivityStageLocked(libraryID, "queued", true, true)
	m.debounceReady[libraryID] = false
	delete(m.lastFlushed, libraryID)
	delete(m.lastActivityAt, libraryID)
	m.scheduleDebounceLocked(libraryID)
	result := m.statusLocked(libraryID)
	m.mu.Unlock()

	m.flushAllStatuses(true)
	return result
}

func (m *LibraryScanManager) clearRetryStateLocked(libraryID int, status *libraryScanStatus) {
	if status == nil {
		return
	}
	status.RetryCount = 0
	status.NextRetryAt = ""
	status.Error = ""
	status.LastError = ""
	if timer, ok := m.retryTimers[libraryID]; ok {
		timer.Stop()
		delete(m.retryTimers, libraryID)
	}
}

func mergeScanStartRequest(current, incoming scanStartRequest) scanStartRequest {
	return scanStartRequest{
		identify: current.identify || incoming.identify,
		subpaths: mergeScanSubpaths(current.subpaths, incoming.subpaths),
	}
}

func mergeScanSubpaths(current, incoming []string) []string {
	if len(current) == 0 {
		if len(incoming) == 0 {
			return nil
		}
		return append([]string(nil), incoming...)
	}
	if len(incoming) == 0 {
		return nil
	}
	merged := append(append([]string(nil), current...), incoming...)
	normalized, err := db.NormalizeScanSubpaths(merged)
	if err != nil {
		return nil
	}
	return normalized
}

func (m *LibraryScanManager) scheduleDebounceLocked(libraryID int) {
	if timer, ok := m.debounceTimers[libraryID]; ok {
		timer.Stop()
	}
	m.debounceTimers[libraryID] = time.AfterFunc(libraryScanDebounceWindow, func() {
		m.mu.Lock()
		delete(m.debounceTimers, libraryID)
		m.debounceReady[libraryID] = true
		m.mu.Unlock()
		m.scheduleNext()
	})
}

func (m *LibraryScanManager) setActivityStageLocked(libraryID int, stage string, resetCurrent bool, clearRecent bool) {
	activity := m.activities[libraryID]
	activity.Stage = stage
	if clearRecent {
		activity.Recent = nil
	}
	if resetCurrent {
		activity.Current = nil
	}
	if activity.Recent == nil {
		activity.Recent = []libraryScanActivityEntry{}
	}
	m.activities[libraryID] = activity
}

func (m *LibraryScanManager) clearActivityLocked(libraryID int) {
	delete(m.activities, libraryID)
	delete(m.lastActivityAt, libraryID)
	if timer, ok := m.activityTimers[libraryID]; ok {
		timer.Stop()
		delete(m.activityTimers, libraryID)
	}
}

func (m *LibraryScanManager) relativeActivityPathLocked(libraryID int, path string) string {
	root := m.paths[libraryID]
	if path == "" || root == "" {
		return ""
	}
	relPath, err := filepath.Rel(root, path)
	if err != nil {
		return ""
	}
	if relPath == "." {
		return ""
	}
	return relPath
}

func (m *LibraryScanManager) recordActivity(libraryID int, phase string, target string, path string) {
	m.mu.Lock()
	if _, ok := m.jobs[libraryID]; !ok {
		m.mu.Unlock()
		return
	}
	relativePath := m.relativeActivityPathLocked(libraryID, path)
	entry := libraryScanActivityEntry{
		Phase:        phase,
		Target:       target,
		RelativePath: relativePath,
		At:           time.Now().UTC().Format(time.RFC3339),
	}
	activity := m.activities[libraryID]
	switch phase {
	case "discovery":
		activity.Stage = "discovery"
	case "enrichment":
		activity.Stage = "enrichment"
	case "identify":
		activity.Stage = "identify"
	}
	activity.Current = &entry
	activity.Recent = append([]libraryScanActivityEntry{entry}, activity.Recent...)
	if len(activity.Recent) > libraryScanActivityMaxRecent {
		activity.Recent = activity.Recent[:libraryScanActivityMaxRecent]
	}
	m.activities[libraryID] = activity
	m.mu.Unlock()
	m.flushActivity(libraryID, false)
}

func (m *LibraryScanManager) RecordIdentifyActivity(libraryID int, path string) {
	m.recordActivity(libraryID, "identify", "file", path)
}

func (m *LibraryScanManager) finalizeActivityLocked(libraryID int, status libraryScanStatus) {
	status.EnrichmentPhase = normalizeEnrichmentPhase(status.EnrichmentPhase, status.Enriching)
	identifyActive := status.IdentifyPhase == libraryIdentifyPhaseQueued || status.IdentifyPhase == libraryIdentifyPhaseIdentifying
	if status.Phase == libraryScanPhaseFailed || (!identifyActive && status.IdentifyPhase == libraryIdentifyPhaseFailed) {
		m.setActivityStageLocked(libraryID, "failed", true, true)
		return
	}
	if status.EnrichmentPhase != libraryEnrichmentPhaseIdle || identifyActive || status.Phase == libraryScanPhaseQueued || status.Phase == libraryScanPhaseScanning {
		return
	}
	m.clearActivityLocked(libraryID)
}

func (m *LibraryScanManager) flushActivity(libraryID int, force bool) {
	m.mu.Lock()
	if _, ok := m.jobs[libraryID]; !ok {
		m.mu.Unlock()
		return
	}
	now := time.Now()
	if !force {
		if last := m.lastActivityAt[libraryID]; !last.IsZero() && now.Sub(last) < libraryScanActivityFlushInterval {
			if _, ok := m.activityTimers[libraryID]; !ok {
				delay := libraryScanActivityFlushInterval - now.Sub(last)
				m.activityTimers[libraryID] = time.AfterFunc(delay, func() {
					m.mu.Lock()
					delete(m.activityTimers, libraryID)
					m.mu.Unlock()
					m.flushActivity(libraryID, true)
				})
			}
			m.mu.Unlock()
			return
		}
	}
	if timer, ok := m.activityTimers[libraryID]; ok {
		timer.Stop()
		delete(m.activityTimers, libraryID)
	}
	m.lastActivityAt[libraryID] = now
	status := m.statusLocked(libraryID)
	m.mu.Unlock()
	m.broadcast(status)
}

func (m *LibraryScanManager) status(libraryID int) libraryScanStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.statusLocked(libraryID)
}

func (m *LibraryScanManager) ownerID(libraryID int) int {
	m.mu.Lock()
	if ownerID, ok := m.owners[libraryID]; ok {
		m.mu.Unlock()
		return ownerID
	}
	dbConn := m.db
	m.mu.Unlock()
	if dbConn == nil {
		return 0
	}

	var ownerID int
	if err := dbConn.QueryRow(`SELECT user_id FROM libraries WHERE id = ?`, libraryID).Scan(&ownerID); err != nil {
		return 0
	}

	m.mu.Lock()
	if _, ok := m.owners[libraryID]; !ok {
		m.owners[libraryID] = ownerID
	}
	m.mu.Unlock()
	return ownerID
}

func (m *LibraryScanManager) scanSubpaths(libraryID int) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	subpaths := m.subpaths[libraryID]
	if len(subpaths) == 0 {
		return nil
	}
	return append([]string(nil), subpaths...)
}

func (m *LibraryScanManager) statusLocked(libraryID int) libraryScanStatus {
	if status, ok := m.jobs[libraryID]; ok {
		status.EnrichmentPhase = normalizeEnrichmentPhase(status.EnrichmentPhase, status.Enriching)
		if status.MaxRetries <= 0 {
			status.MaxRetries = 3
		}
		status.QueuePosition = m.queuePositionLocked(libraryID)
		if status.Error == "" {
			status.Error = scanStatusWarning(status, m.paths[libraryID])
		}
		if activity, ok := m.activities[libraryID]; ok {
			status.Activity = cloneLibraryScanActivity(activity)
		}
		return status
	}
	return libraryScanStatus{
		LibraryID:       libraryID,
		Phase:           libraryScanPhaseIdle,
		EnrichmentPhase: libraryEnrichmentPhaseIdle,
		Enriching:       false,
		IdentifyPhase:   libraryIdentifyPhaseIdle,
		EstimatedItems:  0,
		MaxRetries:      3,
		QueuePosition:   0,
	}
}

func scanStatusWarning(status libraryScanStatus, path string) string {
	if status.Phase == libraryScanPhaseCompleted && status.Processed == 0 && path != "" {
		return fmt.Sprintf(
			"No media files were found under %s. Verify the mounted library path contains media and that PLUM_MEDIA_*_PATH in .env points to the correct host folder.",
			path,
		)
	}
	return ""
}

func (m *LibraryScanManager) scheduleNext() {
	m.mu.Lock()
	if m.activeScanID != 0 {
		m.mu.Unlock()
		return
	}
	nextID, status, libraryType, path := m.nextQueuedLocked()
	if nextID == 0 {
		m.mu.Unlock()
		return
	}
	status.Phase = libraryScanPhaseScanning
	status.QueuePosition = 0
	status.FinishedAt = ""
	status.Error = ""
	status.NextRetryAt = ""
	m.jobs[nextID] = status
	m.setActivityStageLocked(nextID, "discovery", true, false)
	m.activeScanID = nextID
	m.mu.Unlock()

	m.flushAllStatuses(true)
	go m.run(nextID, status, libraryType, path)
}

func (m *LibraryScanManager) nextQueuedLocked() (int, libraryScanStatus, string, string) {
	queued := m.readyQueuedLibrariesLocked()
	if len(queued) == 0 {
		return 0, libraryScanStatus{}, "", ""
	}
	nextID := queued[0].id
	status := m.jobs[nextID]
	return nextID, status, m.types[nextID], m.paths[nextID]
}

// libraryScanQueueTypePriority orders the global scan queue so video libraries (and identify)
// run before music, matching product expectation: movies → TV → anime → music.
func libraryScanQueueTypePriority(libraryType string) int {
	switch libraryType {
	case db.LibraryTypeMovie:
		return 0
	case db.LibraryTypeTV:
		return 1
	case db.LibraryTypeAnime:
		return 2
	case db.LibraryTypeMusic:
		return 3
	default:
		return 4
	}
}

func (m *LibraryScanManager) queuedLibrariesLocked() []queuedLibrary {
	queued := make([]queuedLibrary, 0, len(m.jobs))
	for libraryID, status := range m.jobs {
		if status.Phase != libraryScanPhaseQueued {
			continue
		}
		queued = append(queued, queuedLibrary{
			id:       libraryID,
			queuedAt: status.QueuedAt,
		})
	}
	sort.Slice(queued, func(i, j int) bool {
		pi := libraryScanQueueTypePriority(m.types[queued[i].id])
		pj := libraryScanQueueTypePriority(m.types[queued[j].id])
		if pi != pj {
			return pi < pj
		}
		if queued[i].queuedAt != queued[j].queuedAt {
			if queued[i].queuedAt == "" {
				return false
			}
			if queued[j].queuedAt == "" {
				return true
			}
			return queued[i].queuedAt < queued[j].queuedAt
		}
		return queued[i].id < queued[j].id
	})
	return queued
}

func (m *LibraryScanManager) readyQueuedLibrariesLocked() []queuedLibrary {
	all := m.queuedLibrariesLocked()
	ready := make([]queuedLibrary, 0, len(all))
	for _, item := range all {
		if !m.debounceReady[item.id] {
			continue
		}
		ready = append(ready, item)
	}
	return ready
}

func (m *LibraryScanManager) queuePositionLocked(libraryID int) int {
	queued := m.queuedLibrariesLocked()
	for idx, item := range queued {
		if item.id == libraryID {
			return idx + 1
		}
	}
	return 0
}

func (m *LibraryScanManager) run(libraryID int, status libraryScanStatus, libraryType, path string) {
	subpaths := m.scanSubpaths(libraryID)
	delta, err := scanLibraryDiscovery(context.Background(), m.db, path, libraryType, libraryID, db.ScanOptions{
		ProbeMedia:             false,
		ProbeEmbeddedSubtitles: false,
		ScanSidecarSubtitles:   false,
		Subpaths:               subpaths,
		HashMode:               db.ScanHashModeDefer,
		Progress: func(progress db.ScanProgress) {
			m.updateProgress(libraryID, progress)
		},
		Activity: func(activity db.ScanActivity) {
			m.recordActivity(libraryID, activity.Phase, activity.Target, activity.Path)
		},
	})
	if err != nil {
		m.finish(libraryID, libraryScanPhaseFailed, db.ScanResult{}, err.Error())
		return
	}
	m.finish(libraryID, libraryScanPhaseCompleted, delta.Result, "")
	if status.IdentifyRequested && libraryType != db.LibraryTypeMusic {
		m.startIdentify(libraryID)
	}
	m.startEnrichment(libraryID, libraryType, path, subpaths, delta.TouchedFiles, status.IdentifyRequested)
}

func (m *LibraryScanManager) updateProgress(libraryID int, progress db.ScanProgress) {
	m.mu.Lock()
	status, ok := m.jobs[libraryID]
	if !ok {
		m.mu.Unlock()
		return
	}
	status.Processed = progress.Processed
	status.Added = progress.Result.Added
	status.Updated = progress.Result.Updated
	status.Removed = progress.Result.Removed
	status.Unmatched = progress.Result.Unmatched
	status.Skipped = progress.Result.Skipped
	m.jobs[libraryID] = status
	m.mu.Unlock()
	m.flushStatus(libraryID, false)
}

func (m *LibraryScanManager) startIdentify(libraryID int) {
	m.mu.Lock()
	status, ok := m.jobs[libraryID]
	if !ok {
		m.mu.Unlock()
		return
	}
	status.IdentifyPhase = libraryIdentifyPhaseQueued
	m.jobs[libraryID] = status
	m.setActivityStageLocked(libraryID, "identify", true, false)
	m.mu.Unlock()
	log.Printf("identify library=%d status=queued", libraryID)
	m.flushStatus(libraryID, true)

	go func() {
		m.identifySem <- struct{}{}
		defer func() { <-m.identifySem }()
		startedAt := time.Now()

		m.mu.Lock()
		status, ok := m.jobs[libraryID]
		if !ok {
			m.mu.Unlock()
			return
		}
		libraryType := m.types[libraryID]
		status.IdentifyPhase = libraryIdentifyPhaseIdentifying
		m.jobs[libraryID] = status
		m.setActivityStageLocked(libraryID, "identify", true, false)
		m.mu.Unlock()
		log.Printf("identify library=%d type=%s status=started", libraryID, libraryType)
		m.flushStatus(libraryID, true)

		handler := m.handler
		if handler == nil {
			handler = &LibraryHandler{DB: m.db, Meta: m.meta, ScanJobs: m}
		} else if handler.ScanJobs == nil {
			handler.ScanJobs = m
		}
		result, err := handler.identifyLibrary(context.Background(), libraryID)

		m.mu.Lock()
		status, ok = m.jobs[libraryID]
		if !ok {
			m.mu.Unlock()
			return
		}
		status.Identified = result.Identified
		status.IdentifyFailed = result.Failed
		if err != nil {
			status.IdentifyPhase = libraryIdentifyPhaseFailed
			status.LastError = err.Error()
		} else if result.Failed > 0 {
			status.IdentifyPhase = libraryIdentifyPhaseFailed
			status.LastError = fmt.Sprintf("%d item(s) could not be identified automatically", result.Failed)
		} else {
			status.IdentifyPhase = libraryIdentifyPhaseCompleted
			m.clearRetryStateLocked(libraryID, &status)
			status.LastError = ""
		}
		m.jobs[libraryID] = status
		m.finalizeActivityLocked(libraryID, status)
		m.mu.Unlock()
		outcome := "completed"
		if err != nil {
			outcome = "failed"
		} else if result.Failed > 0 {
			outcome = "partial-failed"
		}
		log.Printf(
			"identify library=%d type=%s status=%s identified=%d failed=%d elapsed=%s",
			libraryID,
			libraryType,
			outcome,
			result.Identified,
			result.Failed,
			time.Since(startedAt).Round(time.Millisecond),
		)
		m.flushStatus(libraryID, true)
		if result.Identified == 0 {
			if handler != nil && handler.SearchIndex != nil {
				handler.SearchIndex.Queue(libraryID, false)
			}
		}
		if err != nil {
			_ = m.scheduleRetry(libraryID, true, err.Error())
		}
	}()
}

func (m *LibraryScanManager) finish(libraryID int, phase string, result db.ScanResult, errText string) {
	m.mu.Lock()
	status := m.jobs[libraryID]
	status.Phase = phase
	status.Processed = result.Added + result.Updated + result.Skipped
	status.Added = result.Added
	status.Updated = result.Updated
	status.Removed = result.Removed
	status.Unmatched = result.Unmatched
	status.Skipped = result.Skipped
	status.Error = errText
	status.FinishedAt = time.Now().UTC().Format(time.RFC3339)
	if phase == libraryScanPhaseFailed {
		status.EnrichmentPhase = libraryEnrichmentPhaseIdle
		status.Enriching = false
		status.LastError = errText
		m.setActivityStageLocked(libraryID, "failed", true, true)
	} else {
		m.clearRetryStateLocked(libraryID, &status)
	}
	m.jobs[libraryID] = status
	if m.activeScanID == libraryID {
		m.activeScanID = 0
	}
	rerun, hasRerun := m.reruns[libraryID]
	if phase == libraryScanPhaseCompleted && hasRerun {
		delete(m.reruns, libraryID)
		status.Phase = libraryScanPhaseQueued
		status.IdentifyRequested = rerun.identify
		status.QueuedAt = time.Now().UTC().Format(time.RFC3339)
		status.FinishedAt = ""
		status.Error = ""
		status.Processed = 0
		status.Added = 0
		status.Updated = 0
		status.Removed = 0
		status.Unmatched = 0
		status.Skipped = 0
		m.jobs[libraryID] = status
		m.subpaths[libraryID] = rerun.subpaths
		m.setActivityStageLocked(libraryID, "queued", true, true)
		m.debounceReady[libraryID] = false
		m.scheduleDebounceLocked(libraryID)
	} else if phase == libraryScanPhaseFailed {
		m.finalizeActivityLocked(libraryID, status)
	}
	m.mu.Unlock()
	m.flushAllStatuses(true)
	if phase == libraryScanPhaseFailed {
		_ = m.scheduleRetry(libraryID, false, errText)
	}
	if phase == libraryScanPhaseCompleted && !hasRerun && !status.IdentifyRequested {
		if handler := m.handler; handler != nil && handler.SearchIndex != nil {
			handler.SearchIndex.Queue(libraryID, false)
		}
	}
	m.scheduleNext()
}

func (m *LibraryScanManager) startEnrichment(libraryID int, libraryType, path string, subpaths []string, tasks []db.EnrichmentTask, identifyRequested bool) {
	if len(tasks) == 0 {
		m.mu.Lock()
		status, ok := m.jobs[libraryID]
		if ok {
			status.EnrichmentPhase = libraryEnrichmentPhaseIdle
			status.Enriching = false
			m.jobs[libraryID] = status
			m.finalizeActivityLocked(libraryID, status)
		}
		m.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())

	m.mu.Lock()
	status, ok := m.jobs[libraryID]
	if !ok {
		m.mu.Unlock()
		cancel()
		return
	}
	status.EnrichmentPhase = libraryEnrichmentPhaseQueued
	status.Enriching = false
	m.jobs[libraryID] = status
	m.enrichCancels[libraryID] = cancel
	m.setActivityStageLocked(libraryID, "enrichment", true, false)
	m.mu.Unlock()
	m.flushStatus(libraryID, true)

	log.Printf(
		"library scan enrichment queued library_id=%d type=%s tasks=%d identify_requested=%v (global enrichment slot: max 1 library at a time; workers per library=%d)",
		libraryID, libraryType, len(tasks), identifyRequested, db.EnrichmentWorkerCount,
	)

	go func() {
		waitStart := time.Now()
		select {
		case m.enrichSem <- struct{}{}:
		case <-ctx.Done():
			m.finishEnrichment(libraryID)
			return
		}
		waitElapsed := time.Since(waitStart).Round(time.Millisecond)
		if waitElapsed > 0 {
			log.Printf(
				"library scan enrichment acquired_slot library_id=%d type=%s waited=%s",
				libraryID, libraryType, waitElapsed,
			)
		}
		defer func() { <-m.enrichSem }()

		m.mu.Lock()
		status, ok := m.jobs[libraryID]
		if !ok {
			m.mu.Unlock()
			return
		}
		status.EnrichmentPhase = libraryEnrichmentPhaseRunning
		status.Enriching = true
		m.jobs[libraryID] = status
		m.mu.Unlock()
		m.flushStatus(libraryID, true)

		options := db.ScanOptions{
			ProbeMedia:             true,
			ProbeEmbeddedSubtitles: true,
			ScanSidecarSubtitles:   true,
			Subpaths:               subpaths,
			Activity: func(activity db.ScanActivity) {
				m.recordActivity(libraryID, activity.Phase, activity.Target, activity.Path)
			},
		}
		musicIdentify := false
		if libraryType == db.LibraryTypeMusic && identifyRequested {
			if musicIdentifier, ok := m.meta.(metadata.MusicIdentifier); ok {
				options.MusicIdentifier = musicIdentifier
				musicIdentify = true
			}
		}

		runStart := time.Now()
		log.Printf(
			"library scan enrichment running library_id=%d type=%s tasks=%d music_identify=%v subpaths=%d",
			libraryID, libraryType, len(tasks), musicIdentify, len(subpaths),
		)
		err := enrichLibraryTasks(ctx, m.db, path, libraryType, libraryID, tasks, options)
		runElapsed := time.Since(runStart).Round(time.Millisecond)
		if err != nil {
			log.Printf(
				"library scan enrichment finished library_id=%d type=%s elapsed=%s err=%v",
				libraryID, libraryType, runElapsed, err,
			)
			if ctx.Err() == nil {
				m.failEnrichment(libraryID, err.Error())
				return
			}
			m.finishEnrichment(libraryID)
			return
		}
		log.Printf(
			"library scan enrichment finished library_id=%d type=%s elapsed=%s ok",
			libraryID, libraryType, runElapsed,
		)
		m.finishEnrichment(libraryID)
	}()
}

func (m *LibraryScanManager) canIdentifyLibrary(libraryType string) bool {
	if m.meta == nil {
		return false
	}
	if libraryType != db.LibraryTypeMusic {
		return true
	}
	_, ok := m.meta.(metadata.MusicIdentifier)
	return ok
}

func (m *LibraryScanManager) finishEnrichment(libraryID int) {
	m.mu.Lock()
	status, ok := m.jobs[libraryID]
	if !ok {
		m.mu.Unlock()
		return
	}
	status.EnrichmentPhase = libraryEnrichmentPhaseIdle
	status.Enriching = false
	m.jobs[libraryID] = status
	if cancel, ok := m.enrichCancels[libraryID]; ok {
		cancel()
		delete(m.enrichCancels, libraryID)
	}
	m.finalizeActivityLocked(libraryID, status)
	m.mu.Unlock()
	m.flushStatus(libraryID, true)
}

func (m *LibraryScanManager) failEnrichment(libraryID int, errText string) {
	m.mu.Lock()
	status, ok := m.jobs[libraryID]
	if !ok {
		m.mu.Unlock()
		return
	}
	status.Phase = libraryScanPhaseFailed
	status.EnrichmentPhase = libraryEnrichmentPhaseIdle
	status.Enriching = false
	status.Error = errText
	status.LastError = errText
	status.FinishedAt = time.Now().UTC().Format(time.RFC3339)
	m.jobs[libraryID] = status
	m.setActivityStageLocked(libraryID, "failed", true, true)
	if cancel, ok := m.enrichCancels[libraryID]; ok {
		cancel()
		delete(m.enrichCancels, libraryID)
	}
	m.finalizeActivityLocked(libraryID, status)
	m.mu.Unlock()
	m.flushStatus(libraryID, true)
	_ = m.scheduleRetry(libraryID, false, errText)
}

func (m *LibraryScanManager) flushAllStatuses(force bool) {
	m.mu.Lock()
	ids := make([]int, 0, len(m.jobs))
	for libraryID := range m.jobs {
		ids = append(ids, libraryID)
	}
	m.mu.Unlock()
	for _, libraryID := range ids {
		m.flushStatus(libraryID, force)
	}
}

func (m *LibraryScanManager) flushStatus(libraryID int, force bool) {
	m.mu.Lock()
	status, ok := m.jobs[libraryID]
	if !ok {
		m.mu.Unlock()
		return
	}
	status.QueuePosition = m.queuePositionLocked(libraryID)
	if status.Error == "" {
		status.Error = scanStatusWarning(status, m.paths[libraryID])
	}
	last := m.lastFlushed[libraryID]
	now := time.Now()
	shouldFlush := force ||
		last.at.IsZero() ||
		status.Processed-last.processed >= libraryScanProgressFlushEvery ||
		now.Sub(last.at) >= libraryScanProgressFlushInterval
	if !shouldFlush {
		m.mu.Unlock()
		return
	}
	m.lastFlushed[libraryID] = libraryScanFlushState{
		at:        now,
		processed: status.Processed,
	}
	path := m.paths[libraryID]
	libraryType := m.types[libraryID]
	broadcastStatus := m.statusLocked(libraryID)
	m.mu.Unlock()

	_ = db.UpsertLibraryJobStatus(m.db, runtimeLibraryStatusToPersistent(status, path, libraryType))
	m.broadcast(broadcastStatus)
}

func runtimeLibraryStatusToPersistent(status libraryScanStatus, path, libraryType string) db.LibraryJobStatus {
	status.EnrichmentPhase = normalizeEnrichmentPhase(status.EnrichmentPhase, status.Enriching)
	return db.LibraryJobStatus{
		LibraryID:         status.LibraryID,
		Path:              path,
		Type:              libraryType,
		Phase:             status.Phase,
		EnrichmentPhase:   status.EnrichmentPhase,
		Enriching:         status.Enriching,
		IdentifyPhase:     status.IdentifyPhase,
		Identified:        status.Identified,
		IdentifyFailed:    status.IdentifyFailed,
		Processed:         status.Processed,
		Added:             status.Added,
		Updated:           status.Updated,
		Removed:           status.Removed,
		Unmatched:         status.Unmatched,
		Skipped:           status.Skipped,
		IdentifyRequested: status.IdentifyRequested,
		QueuedAt:          status.QueuedAt,
		EstimatedItems:    status.EstimatedItems,
		Error:             status.Error,
		RetryCount:        status.RetryCount,
		MaxRetries:        status.MaxRetries,
		NextRetryAt:       status.NextRetryAt,
		LastError:         status.LastError,
		NextScheduledAt:   status.NextScheduledAt,
		StartedAt:         status.StartedAt,
		FinishedAt:        status.FinishedAt,
	}
}

func persistedLibraryStatusToRuntime(status db.LibraryJobStatus) libraryScanStatus {
	maxRetries := status.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}
	return libraryScanStatus{
		LibraryID:         status.LibraryID,
		Phase:             status.Phase,
		EnrichmentPhase:   normalizeEnrichmentPhase(status.EnrichmentPhase, status.Enriching),
		Enriching:         status.Enriching,
		IdentifyPhase:     status.IdentifyPhase,
		Identified:        status.Identified,
		IdentifyFailed:    status.IdentifyFailed,
		Processed:         status.Processed,
		Added:             status.Added,
		Updated:           status.Updated,
		Removed:           status.Removed,
		Unmatched:         status.Unmatched,
		Skipped:           status.Skipped,
		IdentifyRequested: status.IdentifyRequested,
		QueuedAt:          status.QueuedAt,
		EstimatedItems:    status.EstimatedItems,
		Error:             status.Error,
		RetryCount:        status.RetryCount,
		MaxRetries:        maxRetries,
		NextRetryAt:       status.NextRetryAt,
		LastError:         status.LastError,
		NextScheduledAt:   status.NextScheduledAt,
		StartedAt:         status.StartedAt,
		FinishedAt:        status.FinishedAt,
	}
}

func (m *LibraryScanManager) broadcast(status libraryScanStatus) {
	if m.hub == nil {
		return
	}
	ownerID := m.ownerID(status.LibraryID)
	if ownerID <= 0 {
		return
	}
	payload, err := libraryScanUpdatePayload(status)
	if err != nil {
		return
	}
	m.hub.BroadcastToUser(ownerID, payload)
}

func libraryScanUpdatePayload(status libraryScanStatus) ([]byte, error) {
	return json.Marshal(map[string]any{
		"type": "library_scan_update",
		"scan": status,
	})
}
