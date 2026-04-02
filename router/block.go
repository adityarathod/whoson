package router

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// multifilterState holds the current MULTIFILTER lists from ParentalControl.asp.
type multifilterState struct {
	Enable  []string // "2" = blocked, "0" = inactive
	MAC     []string
	Name    []string
	Daytime []string // per-entry schedule
}

var (
	reEnable  = regexp.MustCompile(`var MULTIFILTER_ENABLE = '([^']*)'`)
	reMAC     = regexp.MustCompile(`var MULTIFILTER_MAC = '([^']*)'`)
	reName    = regexp.MustCompile(`var MULTIFILTER_DEVICENAME = decodeURIComponent\('([^']*)'\)`)
	reDaytime = regexp.MustCompile(`var MULTIFILTER_MACFILTER_DAYTIME_V2 = '([^']*)'`)
)

func (c *Client) fetchMultifilter() (*multifilterState, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/ParentalControl.asp", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Referer", c.baseURL+"/index.asp")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// htmlDecode replaces HTML entities used by the router for > and <.
	htmlDecode := func(s string) string {
		s = strings.ReplaceAll(s, "&#62", ">")
		return strings.ReplaceAll(s, "&#60", "<")
	}

	extract := func(re *regexp.Regexp) []string {
		m := re.FindSubmatch(body)
		if m == nil || string(m[1]) == "" {
			return nil
		}
		return strings.Split(htmlDecode(string(m[1])), ">")
	}

	// DEVICENAME is wrapped in decodeURIComponent(), so > is stored as %3E.
	extractURLEncoded := func(re *regexp.Regexp) []string {
		m := re.FindSubmatch(body)
		if m == nil || string(m[1]) == "" {
			return nil
		}
		decoded, _ := url.QueryUnescape(string(m[1]))
		return strings.Split(decoded, ">")
	}

	return &multifilterState{
		Enable:  extract(reEnable),
		MAC:     extract(reMAC),
		Name:    extractURLEncoded(reName),
		Daytime: extract(reDaytime),
	}, nil
}

func (c *Client) applyMultifilter(state *multifilterState) error {
	join := func(s []string) string { return strings.Join(s, ">") }

	form := url.Values{}
	form.Set("current_page", "ParentalControl.asp")
	form.Set("next_page", "")
	form.Set("modified", "0")
	form.Set("action_wait", "5")
	form.Set("action_mode", "apply")
	form.Set("action_script", "restart_firewall")
	form.Set("MULTIFILTER_ALL", "1")
	form.Set("MULTIFILTER_ENABLE", join(state.Enable))
	form.Set("MULTIFILTER_MAC", join(state.MAC))
	form.Set("MULTIFILTER_DEVICENAME", join(state.Name))
	form.Set("MULTIFILTER_MACFILTER_DAYTIME_V2", join(state.Daytime))
	form.Set("PC_mac", "")

	req, err := http.NewRequest("POST", c.baseURL+"/start_apply.htm", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", c.baseURL+"/ParentalControl.asp")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	// Drain body so the router fully processes the response before we close.
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	return nil
}

// Block sets the given MAC address to blocked in the router's Parental Controls.
// If the entry doesn't exist it is appended; if it exists but is inactive it is re-enabled.
func (c *Client) Block(mac, name string) error {
	state, err := c.fetchMultifilter()
	if err != nil {
		return fmt.Errorf("fetch multifilter state: %w", err)
	}

	for i, m := range state.MAC {
		if strings.EqualFold(m, mac) {
			if i < len(state.Enable) && state.Enable[i] == "2" {
				return fmt.Errorf("%s is already blocked", mac)
			}
			state.Enable[i] = "2"
			return c.applyMultifilter(state)
		}
	}

	// New entry: use existing daytime as template, fall back to a known-good value.
	template := "W03E21000700<W04122000800"
	if len(state.Daytime) > 0 {
		template = state.Daytime[0]
	}

	state.Enable = append(state.Enable, "2")
	state.MAC = append(state.MAC, strings.ToUpper(mac))
	state.Name = append(state.Name, name)
	state.Daytime = append(state.Daytime, template)

	return c.applyMultifilter(state)
}

// Unblock sets the given MAC address to inactive in the router's Parental Controls.
// The entry is kept in the list (posting an empty MAC list is silently ignored by the firmware).
func (c *Client) Unblock(mac string) error {
	state, err := c.fetchMultifilter()
	if err != nil {
		return fmt.Errorf("fetch multifilter state: %w", err)
	}

	for i, m := range state.MAC {
		if strings.EqualFold(m, mac) {
			if i < len(state.Enable) && state.Enable[i] != "2" {
				return fmt.Errorf("%s is not blocked", mac)
			}
			state.Enable[i] = "0"
			return c.applyMultifilter(state)
		}
	}
	return fmt.Errorf("%s is not blocked", mac)
}
