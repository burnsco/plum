package db

import (
	"database/sql"
	"strings"
)

const (
	IntroSkipModeOff    = "off"
	IntroSkipModeManual = "manual"
	IntroSkipModeAuto   = "auto"
)

func NormalizeIntroSkipMode(raw string) string {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case IntroSkipModeOff:
		return IntroSkipModeOff
	case IntroSkipModeAuto:
		return IntroSkipModeAuto
	default:
		return IntroSkipModeManual
	}
}

func GetLibraryIntroSkipMode(dbConn *sql.DB, libraryID int) string {
	if libraryID <= 0 {
		return IntroSkipModeManual
	}
	var raw sql.NullString
	err := dbConn.QueryRow(`SELECT intro_skip_mode FROM libraries WHERE id = ?`, libraryID).Scan(&raw)
	if err != nil || !raw.Valid || strings.TrimSpace(raw.String) == "" {
		return IntroSkipModeManual
	}
	return NormalizeIntroSkipMode(raw.String)
}
