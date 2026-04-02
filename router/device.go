package router

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// flexString unmarshals a JSON value that may be either a string or a number.
type flexString string

func (f *flexString) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*f = flexString(s)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(data, &n); err == nil {
		*f = flexString(n.String())
		return nil
	}
	return fmt.Errorf("cannot unmarshal %s into string", data)
}

// Device represents a single network client as reported by networkmapd.
type Device struct {
	MAC             string `json:"mac"`
	Name            string `json:"name"`
	NickName        string `json:"nickName"`
	IP              string `json:"ip"`
	IPMethod        string `json:"ipMethod"`
	Vendor          string `json:"vendor"`
	IsOnline        string `json:"isOnline"`
	IsWL            string `json:"isWL"` // "0" = wired, "1" = 2.4GHz, "2" = 5GHz
	IsGateway       string `json:"isGateway"`
	RSSI            string `json:"rssi"`
	SSID            string `json:"ssid"`
	CurRx           string `json:"curRx"`
	CurTx           string `json:"curTx"`
	WLConnectTime   string `json:"wlConnectTime"`
	InternetMode    string     `json:"internetMode"`
	InternetState   flexString `json:"internetState"`
	DefaultType     string `json:"defaultType"`
	Type            string `json:"type"`
	From            string `json:"from"`
	AmeshPapMac     string `json:"amesh_papMac"`
	AmeshIsReClient string `json:"amesh_isReClient"`
}

// Devices fetches the current list of network clients from the router.
func (c *Client) Devices() ([]Device, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/update_clients.asp", nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Referer", c.baseURL+"/index.asp")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch devices: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	return parseNetworkmapd(body)
}

// parseNetworkmapd extracts the fromNetworkmapd payload from the response body
// and decodes it into a flat slice of Devices.
//
// The raw format is a JS variable assignment per line, e.g.:
//
//	fromNetworkmapd : [{...},{...}],
//
// Each element is a map of MAC → Device object.
func parseNetworkmapd(body []byte) ([]Device, error) {
	scanner := bufio.NewScanner(bytes.NewReader(body))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, "fromNetworkmapd") {
			continue
		}
		line = strings.ReplaceAll(line, "fromNetworkmapd : ", "")
		line = strings.TrimRight(strings.TrimSpace(line), ",")

		// Array of map[mac]→Device (some values may be arrays; skip those)
		var raw []map[string]json.RawMessage
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			return nil, fmt.Errorf("parse networkmapd JSON: %w", err)
		}

		var devices []Device
		for _, m := range raw {
			for _, v := range m {
				var d Device
				if err := json.Unmarshal(v, &d); err == nil {
					devices = append(devices, d)
				}
			}
		}
		return devices, nil
	}
	return nil, fmt.Errorf("fromNetworkmapd not found in response")
}
