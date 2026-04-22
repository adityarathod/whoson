package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/adityarathod/whoson/oui"
	"github.com/adityarathod/whoson/router"
	"github.com/joho/godotenv"
)

const routerURL = "http://192.168.50.1"

type summary struct {
	MAC          string `json:"mac"`
	Name         string `json:"name"`
	IP           string `json:"ip"`
	Vendor       string `json:"vendor,omitempty"`
	RSSI         string `json:"rssi"`
	Online       bool   `json:"online"`
	Blocked      bool   `json:"blocked"`
	Connectivity string `json:"connectivity"`
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

func boolStr(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func printTable(rows []summary) {
	headers := []string{"MAC", "NAME", "IP", "VENDOR", "RSSI", "ONLINE", "BLOCKED", "CONNECTIVITY"}

	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, r := range rows {
		cols := []string{r.MAC, r.Name, r.IP, r.Vendor, r.RSSI, boolStr(r.Online), boolStr(r.Blocked), r.Connectivity}
		for i, c := range cols {
			if len(c) > widths[i] {
				widths[i] = len(c)
			}
		}
	}

	sep := func() {
		fmt.Print("+")
		for _, w := range widths {
			fmt.Print(strings.Repeat("-", w+2) + "+")
		}
		fmt.Println()
	}

	row := func(cols []string) {
		fmt.Print("|")
		for i, c := range cols {
			fmt.Printf(" %-*s |", widths[i], c)
		}
		fmt.Println()
	}

	sep()
	row(headers)
	sep()
	for _, r := range rows {
		row([]string{r.MAC, r.Name, r.IP, r.Vendor, r.RSSI, boolStr(r.Online), boolStr(r.Blocked), r.Connectivity})
	}
	sep()
}

func summarize(devices []router.Device, db oui.DB) []summary {
	results := make([]summary, len(devices))
	for i, d := range devices {
		name := d.NickName
		if name == "" {
			name = d.Name
		}
		vendor := db.Lookup(d.MAC)
		if vendor == "" {
			vendor = d.Vendor
		}
		results[i] = summary{
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

func main() {
	outputFormat := flag.String("output-format", "json", "output format: json or table")
	blockMAC := flag.String("block", "", "block a device by MAC address")
	unblockMAC := flag.String("unblock", "", "unblock a device by MAC address")
	flag.Parse()

	if *outputFormat != "json" && *outputFormat != "table" {
		fmt.Fprintf(os.Stderr, "invalid --output-format %q: must be json or table\n", *outputFormat)
		os.Exit(1)
	}

	if err := godotenv.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "note: could not load .env: %v\n", err)
	}

	username := os.Getenv("R_USER")
	password := os.Getenv("R_PASSWORD")
	if username == "" || password == "" {
		fmt.Fprintln(os.Stderr, "R_USER and R_PASSWORD must be set (via env or .env file)")
		os.Exit(1)
	}

	var db oui.DB
	if path := os.Getenv("OUI_DB"); path != "" {
		var err error
		if db, err = oui.Load(path); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not load OUI DB: %v\n", err)
		}
	}

	r := router.New(routerURL)
	if err := r.Login(username, password); err != nil {
		fmt.Fprintf(os.Stderr, "login: %v\n", err)
		os.Exit(1)
	}

	devices, err := r.Devices()
	if err != nil {
		fmt.Fprintf(os.Stderr, "fetch devices: %v\n", err)
		os.Exit(1)
	}

	if *unblockMAC != "" {
		if err := r.Unblock(*unblockMAC); err != nil {
			fmt.Fprintf(os.Stderr, "unblock: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("unblocked %s\n", *unblockMAC)
		return
	}

	if *blockMAC != "" {
		name := *blockMAC
		for _, d := range devices {
			if strings.EqualFold(d.MAC, *blockMAC) {
				if d.NickName != "" {
					name = d.NickName
				} else if d.Name != "" {
					name = d.Name
				}
				break
			}
		}
		if err := r.Block(*blockMAC, name); err != nil {
			fmt.Fprintf(os.Stderr, "block: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("blocked %s (%s)\n", *blockMAC, name)
		return
	}

	results := summarize(devices, db)
	switch *outputFormat {
	case "table":
		printTable(results)
	default:
		out, _ := json.MarshalIndent(results, "", "  ")
		fmt.Println(string(out))
	}
}
