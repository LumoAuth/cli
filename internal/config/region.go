package config

import (
	"net/url"
	"strings"
)

// Region mirrors the mobile app's deployment selector
// (lumo_push_auth_app/lib/config/region.dart) so the two clients present a
// consistent enrollment experience.
type Region struct {
	ID          string
	Label       string
	Description string
	ServerURL   string
}

var (
	RegionUS = Region{
		ID:          "us",
		Label:       "United States",
		Description: "app.lumoauth.dev",
		ServerURL:   "https://app.lumoauth.dev",
	}
	RegionEU = Region{
		ID:          "eu",
		Label:       "European Union",
		Description: "eu.app.lumoauth.dev",
		ServerURL:   "https://eu.app.lumoauth.dev",
	}
	RegionCustom = Region{
		ID:          "custom",
		Label:       "Other / Self-hosted",
		Description: "Enter a custom server URL (e.g. local dev instance)",
		ServerURL:   "",
	}
)

// Regions is the ordered list shown to users when picking a deployment.
var Regions = []Region{RegionUS, RegionEU, RegionCustom}

// RegionForURL returns the matching standard Region for url, or RegionCustom
// if it doesn't match a known deployment. Used to pre-select a default in
// re-prompts.
func RegionForURL(rawURL string) Region {
	if rawURL == "" {
		return RegionUS
	}
	normalized := strings.TrimSuffix(strings.TrimSpace(rawURL), "/")
	switch normalized {
	case RegionUS.ServerURL:
		return RegionUS
	case RegionEU.ServerURL:
		return RegionEU
	default:
		return RegionCustom
	}
}

// IsLocalURL reports whether u points at a developer's local instance — used
// to auto-enable --insecure when the user picks Custom and types a localhost
// URL with a self-signed cert.
func IsLocalURL(u string) bool {
	parsed, err := url.Parse(strings.TrimSpace(u))
	if err != nil || parsed.Host == "" {
		return false
	}
	host := parsed.Hostname()
	if parsed.Scheme == "http" {
		return true
	}
	switch host {
	case "localhost", "127.0.0.1", "::1", "0.0.0.0":
		return true
	}
	return strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local")
}
