package logging

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"time"
)

type Fields map[string]any

func Event(subsystem string, event string, fields Fields) {
	parts := []string{
		"ts=" + time.Now().UTC().Format(time.RFC3339),
		"subsystem=" + quote(subsystem),
		"event=" + quote(event),
	}
	if len(fields) > 0 {
		keys := make([]string, 0, len(fields))
		for key, value := range fields {
			if value == nil {
				continue
			}
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			parts = append(parts, key+"="+quote(fmt.Sprint(fields[key])))
		}
	}
	log.Printf("plum %s", strings.Join(parts, " "))
}

func quote(value string) string {
	if value == "" {
		return `""`
	}
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\r", `\r`, "\t", `\t`)
	return `"` + replacer.Replace(value) + `"`
}
