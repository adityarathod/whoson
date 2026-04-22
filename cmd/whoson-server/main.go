package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/adityarathod/whoson/oui"
	"github.com/adityarathod/whoson/router"
	"github.com/joho/godotenv"
)

const defaultRouterURL = "http://192.168.50.1"

//go:embed templates/*.html
var templateFS embed.FS

var tmpl = template.Must(template.New("").ParseFS(templateFS, "templates/*.html"))

type indexData struct {
	Devices     []deviceSummary
	DevicesJSON template.JS
	Online      int
	Offline     int
	Blocked     int
	Total       int
}

type deviceSummary struct {
	MAC          string `json:"mac"`
	Name         string `json:"name"`
	IP           string `json:"ip"`
	Vendor       string `json:"vendor,omitempty"`
	RSSI         string `json:"rssi"`
	Online       bool   `json:"online"`
	Blocked      bool   `json:"blocked"`
	Connectivity string `json:"connectivity"`
}

type server struct {
	username  string
	password  string
	routerURL string
	db        oui.DB
}

func (s *server) newSession() (*router.Client, error) {
	r := router.New(s.routerURL)
	if err := r.Login(s.username, s.password); err != nil {
		return nil, fmt.Errorf("login: %w", err)
	}
	return r, nil
}

func connectivity(isWL string) string {
	switch isWL {
	case "1":
		return "wlan-24"
	case "2":
		return "wlan-5"
	default:
		return "wired"
	}
}

func (s *server) summarize(devices []router.Device) []deviceSummary {
	results := make([]deviceSummary, len(devices))
	for i, d := range devices {
		name := d.NickName
		if name == "" {
			name = d.Name
		}
		vendor := s.db.Lookup(d.MAC)
		if vendor == "" {
			vendor = d.Vendor
		}
		results[i] = deviceSummary{
			MAC:          d.MAC,
			Name:         name,
			IP:           d.IP,
			Vendor:       vendor,
			RSSI:         strings.TrimSpace(d.RSSI),
			Online:       d.IsOnline == "1",
			Blocked:      d.InternetMode == "block",
			Connectivity: connectivity(d.IsWL),
		}
	}
	return results
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func (s *server) handleIndex(w http.ResponseWriter, r *http.Request) {
	session, err := s.newSession()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	devices, err := session.Devices()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	summaries := s.summarize(devices)
	jsonBytes, _ := json.Marshal(summaries)

	sort.Slice(summaries, func(i, j int) bool {
		a, b := summaries[i], summaries[j]
		if a.Online != b.Online {
			return a.Online
		}
		na := strings.ToLower(a.Name)
		if na == "" {
			na = strings.ToLower(a.MAC)
		}
		nb := strings.ToLower(b.Name)
		if nb == "" {
			nb = strings.ToLower(b.MAC)
		}
		return na < nb
	})

	online, blocked := 0, 0
	for _, d := range summaries {
		if d.Online {
			online++
		}
		if d.Blocked {
			blocked++
		}
	}

	data := indexData{
		Devices:     summaries,
		DevicesJSON: template.JS(jsonBytes),
		Online:      online,
		Offline:     len(summaries) - online,
		Blocked:     blocked,
		Total:       len(summaries),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *server) handleDevices(w http.ResponseWriter, r *http.Request) {
	session, err := s.newSession()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	devices, err := session.Devices()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, s.summarize(devices))
}

func (s *server) handleBlock(w http.ResponseWriter, r *http.Request) {
	mac := r.PathValue("mac")

	session, err := s.newSession()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	name := mac
	if devices, err := session.Devices(); err == nil {
		for _, d := range devices {
			if strings.EqualFold(d.MAC, mac) {
				if d.NickName != "" {
					name = d.NickName
				} else if d.Name != "" {
					name = d.Name
				}
				break
			}
		}
	}

	if err := session.Block(mac, name); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"mac": mac, "status": "blocked"})
}

func (s *server) handleUnblock(w http.ResponseWriter, r *http.Request) {
	mac := r.PathValue("mac")

	session, err := s.newSession()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := session.Unblock(mac); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"mac": mac, "status": "unblocked"})
}

func main() {
	if _, err := os.Stat(".env"); err == nil {
		if err := godotenv.Load(); err != nil {
			log.Printf("note: could not load .env: %v", err)
		}
	}

	username := os.Getenv("R_USER")
	password := os.Getenv("R_PASSWORD")
	if username == "" || password == "" {
		log.Fatal("R_USER and R_PASSWORD must be set (via env or .env file)")
	}

	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	var db oui.DB
	if path := os.Getenv("OUI_DB"); path != "" {
		var err error
		if db, err = oui.Load(path); err != nil {
			log.Printf("warning: could not load OUI DB: %v", err)
		}
	}

	routerURL := os.Getenv("ROUTER_URL")
	if routerURL == "" {
		routerURL = defaultRouterURL
	}

	s := &server{username: username, password: password, routerURL: routerURL, db: db}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.handleIndex)
	mux.HandleFunc("GET /api/devices", s.handleDevices)
	mux.HandleFunc("POST /api/devices/{mac}/block", s.handleBlock)
	mux.HandleFunc("POST /api/devices/{mac}/unblock", s.handleUnblock)

	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
