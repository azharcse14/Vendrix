package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// SiteverifyURL: Cloudflare Turnstile er server-side check (12.2 Website 1).
const SiteverifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

// Turnstile: siteverify client. URL khali hole SiteverifyURL, Client nil hole 10 second timeout.
type Turnstile struct {
	URL    string
	Secret string
	Client *http.Client
}

// Verify (10.11): token Cloudflare e pathay (secret, response, remoteip); success ar hostname = shop er domain
// (X-Shop-Host) hole-i true, jate onno site e solve kora token ekhane na chole. Network ba Cloudflare er bhul e
// error (caller 500 dey; token bhul bole na).
func (t Turnstile) Verify(ctx context.Context, token, remoteIP, hostname string) (bool, error) {
	if token == "" || hostname == "" {
		return false, nil
	}
	u, c := t.URL, t.Client
	if u == "" {
		u = SiteverifyURL
	}
	if c == nil {
		c = &http.Client{Timeout: 10 * time.Second}
	}
	form := url.Values{"secret": {t.Secret}, "response": {token}}
	if remoteIP != "" {
		form.Set("remoteip", remoteIP)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, strings.NewReader(form.Encode()))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.Do(req)
	if err != nil {
		return false, fmt.Errorf("turnstile siteverify: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("turnstile siteverify: HTTP %d", resp.StatusCode)
	}
	var out struct {
		Success  bool   `json:"success"`
		Hostname string `json:"hostname"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return false, fmt.Errorf("turnstile siteverify: %w", err)
	}
	return out.Success && strings.EqualFold(out.Hostname, hostname), nil
}
