package dotenv

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// CandidateRelativePaths matches cmd/plum load order: try .env then ../.env from cwd.
var CandidateRelativePaths = []string{".env", "../.env"}

const envDotenvPathOverride = "PLUM_DOTENV_PATH"

// LoadIntoOSEnv parses PLUM_DOTENV_PATH when set and the file exists, otherwise each candidate
// relative to the working directory. Sets os.Getenv for keys not already defined in the process
// environment. Returns whether any file was read.
func LoadIntoOSEnv() bool {
	if p := strings.TrimSpace(os.Getenv(envDotenvPathOverride)); p != "" {
		abs, err := filepath.Abs(p)
		if err == nil {
			if _, err := os.Stat(abs); err == nil {
				return loadFileIntoEnv(abs)
			}
		}
	}
	loaded := false
	for _, rel := range CandidateRelativePaths {
		if loadFileIntoEnv(rel) {
			loaded = true
		}
	}
	return loaded
}

func loadFileIntoEnv(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		if len(value) >= 2 {
			if (strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"")) ||
				(strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'")) {
				value = value[1 : len(value)-1]
			}
		}
		_ = os.Setenv(key, value)
	}
	return true
}

// ResolveExistingPath returns the absolute path of the first candidate .env that exists.
func ResolveExistingPath() (abs string, ok bool) {
	if p := strings.TrimSpace(os.Getenv(envDotenvPathOverride)); p != "" {
		abs, err := filepath.Abs(p)
		if err != nil {
			return "", false
		}
		abs = filepath.Clean(abs)
		if _, err := os.Stat(abs); err == nil {
			return abs, true
		}
		return "", false
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", false
	}
	for _, rel := range CandidateRelativePaths {
		p := filepath.Join(wd, rel)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return filepath.Clean(p), true
		}
	}
	return "", false
}

// ResolveWritePath returns an absolute path for persisting .env: first existing candidate,
// otherwise .env in the current working directory.
func ResolveWritePath() (string, error) {
	if p := strings.TrimSpace(os.Getenv(envDotenvPathOverride)); p != "" {
		return filepath.Abs(p)
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for _, rel := range CandidateRelativePaths {
		p := filepath.Join(wd, rel)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return filepath.Clean(p), nil
		}
	}
	return filepath.Clean(filepath.Join(wd, ".env")), nil
}

// IsWritablePath reports whether the directory allows new files and, if path exists, the file is writable.
func IsWritablePath(abs string) bool {
	if abs == "" {
		return false
	}
	dir := filepath.Dir(abs)
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return false
	}
	tmp, err := os.CreateTemp(dir, ".plum-env-write-test-*")
	if err != nil {
		return false
	}
	_ = tmp.Close()
	_ = os.Remove(tmp.Name())

	if _, err := os.Stat(abs); os.IsNotExist(err) {
		return true
	}
	wf, err := os.OpenFile(abs, os.O_WRONLY, 0)
	if err != nil {
		return false
	}
	_ = wf.Close()
	return true
}

// ReadKeyValues returns KEY=value pairs from the file for keys in want (empty map if file missing).
func ReadKeyValues(abs string, want map[string]struct{}) map[string]string {
	out := map[string]string{}
	if abs == "" || len(want) == 0 {
		return out
	}
	file, err := os.Open(abs)
	if err != nil {
		return out
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if _, need := want[key]; !need {
			continue
		}
		value = strings.TrimSpace(value)
		if len(value) >= 2 {
			if (strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"")) ||
				(strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'")) {
				value = value[1 : len(value)-1]
			}
		}
		out[key] = value
	}
	return out
}

// Upsert updates or appends KEY=value lines; empty value removes a matching assignment line.
func Upsert(abs string, updates map[string]string) error {
	if abs == "" {
		return os.ErrInvalid
	}
	_ = os.MkdirAll(filepath.Dir(abs), 0o755)

	var lines []string
	if raw, err := os.ReadFile(abs); err == nil && len(raw) > 0 {
		lines = strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
	}

	// Strip trailing empty slice from final newline
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	indexByKey := map[string]int{}
	for i, line := range lines {
		k := parseEnvKeyLine(line)
		if k != "" {
			indexByKey[k] = i
		}
	}

	for key, value := range updates {
		if idx, ok := indexByKey[key]; ok {
			if value == "" {
				lines[idx] = ""
			} else {
				lines[idx] = key + "=" + formatEnvValue(value)
			}
			continue
		}
		if value == "" {
			continue
		}
		lines = append(lines, key+"="+formatEnvValue(value))
	}

	var kept []string
	for _, line := range lines {
		if line != "" {
			kept = append(kept, line)
		}
	}
	body := strings.Join(kept, "\n")
	if len(body) > 0 {
		body += "\n"
	}
	return os.WriteFile(abs, []byte(body), 0o600)
}

func parseEnvKeyLine(line string) string {
	s := strings.TrimSpace(line)
	if s == "" || strings.HasPrefix(s, "#") {
		return ""
	}
	if strings.HasPrefix(s, "export ") {
		s = strings.TrimSpace(strings.TrimPrefix(s, "export "))
	}
	key, _, ok := strings.Cut(s, "=")
	if !ok {
		return ""
	}
	return strings.TrimSpace(key)
}

func formatEnvValue(v string) string {
	if v == "" {
		return ""
	}
	if strings.ContainsAny(v, " \t#\"'") || strings.Contains(v, "\n") {
		return `"` + strings.ReplaceAll(strings.ReplaceAll(v, `\`, `\\`), `"`, `\"`) + `"`
	}
	return v
}
