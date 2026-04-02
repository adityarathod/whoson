package router

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
)

// Client manages an authenticated session with an ASUS router.
type Client struct {
	baseURL string
	http    *http.Client
}

// New returns a Client for the router at baseURL (e.g. "http://192.168.50.1").
func New(baseURL string) *Client {
	jar, _ := cookiejar.New(nil)
	return &Client{
		baseURL: baseURL,
		http: &http.Client{
			Jar: jar,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

// Login authenticates with the router and stores the session cookie.
func (c *Client) Login(username, password string) error {
	loginAuth := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))

	form := url.Values{}
	form.Set("group_id", "")
	form.Set("action_mode", "")
	form.Set("action_script", "")
	form.Set("action_wait", "5")
	form.Set("current_page", "Main_Login.asp")
	form.Set("next_page", "")
	form.Set("login_authorization", loginAuth)
	form.Set("login_captcha", "")

	req, err := http.NewRequest("POST", c.baseURL+"/login.cgi", strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("build login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", c.baseURL+"/Main_Login.asp")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("login request: %w", err)
	}
	resp.Body.Close()

	base, _ := url.Parse(c.baseURL)
	for _, cookie := range c.http.Jar.Cookies(base) {
		if cookie.Name == "asus_token" {
			return nil
		}
	}
	return fmt.Errorf("no asus_token cookie after login — bad credentials?")
}
