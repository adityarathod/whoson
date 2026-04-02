package oui

import (
	"encoding/json"
	"os"
	"strings"
)

// DB is an OUI lookup map keyed by 6 hex digits (e.g. "A4C1E8").
type DB map[string]string

// Load reads the OUI JSON file at path and returns a lookup map.
func Load(path string) (DB, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var db DB
	if err := json.NewDecoder(f).Decode(&db); err != nil {
		return nil, err
	}
	return db, nil
}

// Lookup returns the vendor for a MAC address, or empty string if not found.
func (db DB) Lookup(mac string) string {
	if db == nil {
		return ""
	}
	// Normalise "A4:C1:E8:F5:31:D1" → "A4C1E8"
	parts := strings.SplitN(strings.ToUpper(mac), ":", 4)
	if len(parts) < 3 {
		return ""
	}
	return db[parts[0]+parts[1]+parts[2]]
}
