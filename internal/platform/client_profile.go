package platform

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"regexp"
	"strings"
	"sync/atomic"
	"time"
)

// ClientProfile is fixed for the control process. Web identity and Codex
// identity are independent; credentials and account device IDs stay session-owned.
type ClientProfile struct {
	BrowserUserAgent string
	AcceptLanguage   string
	Timezone         string
	WebVersion       string
	WebBuild         string
	CodexUserAgent   string
	location         *time.Location
}

var configuredClientProfile atomic.Pointer[ClientProfile]

var defaultClientProfile = ClientProfile{
	BrowserUserAgent: "Mozilla/5.0",
	AcceptLanguage:   "en-US,en;q=0.9",
	Timezone:         "UTC",
	CodexUserAgent:   CodexTokenUA,
	location:         time.UTC,
}

var chromiumVersionPattern = regexp.MustCompile(`\b[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+\b`)
var languagePattern = regexp.MustCompile(`^[A-Za-z]{2,8}(?:-[A-Za-z0-9]{1,8})*$`)
var codexUserAgentPattern = regexp.MustCompile(`^codex_cli_rs/[0-9]+\.[0-9]+\.[0-9]+(?:[-+][A-Za-z0-9.-]+)?(?: .*)?$`)

// ConfigureClientProfile loads deployment settings before any upstream traffic.
// Empty browser UA uses the installed Chromium version, rather than a shared
// Windows/Chrome sample or the project's name.
func ConfigureClientProfile(ctx context.Context, getenv func(string) string) error {
	profile := defaultClientProfile
	settings := []struct {
		name  string
		value *string
	}{
		{"TSW_CLIENT_USER_AGENT", &profile.BrowserUserAgent},
		{"TSW_CLIENT_ACCEPT_LANGUAGE", &profile.AcceptLanguage},
		{"TSW_CLIENT_TIMEZONE", &profile.Timezone},
		{"TSW_CLIENT_WEB_VERSION", &profile.WebVersion},
		{"TSW_CLIENT_WEB_BUILD", &profile.WebBuild},
		{"TSW_CLIENT_CODEX_USER_AGENT", &profile.CodexUserAgent},
	}
	for _, setting := range settings {
		value := strings.TrimSpace(getenv(setting.name))
		if value == "" {
			continue
		}
		if !validClientSetting(value) {
			return fmt.Errorf("invalid %s", setting.name)
		}
		*setting.value = value
	}
	if !languagePattern.MatchString(profile.language()) || !codexUserAgentPattern.MatchString(profile.CodexUserAgent) {
		return errors.New("invalid client language or Codex user agent")
	}
	var err error
	profile.location, err = time.LoadLocation(profile.Timezone)
	if err != nil {
		return errors.New("invalid TSW_CLIENT_TIMEZONE")
	}
	if strings.TrimSpace(getenv("TSW_CLIENT_USER_AGENT")) == "" {
		bounded, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		version, err := exec.CommandContext(bounded, "/usr/bin/chromium", "--version").Output()
		if err != nil {
			return errors.New("installed Chromium version is unavailable")
		}
		matched := chromiumVersionPattern.FindString(string(version))
		if matched == "" {
			return errors.New("installed Chromium version is invalid")
		}
		// Chromium reduces the HTTP UA to the major version on desktop Linux.
		major := strings.SplitN(matched, ".", 2)[0]
		profile.BrowserUserAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/" + major + ".0.0.0 Safari/537.36"
	}
	configuredClientProfile.Store(&profile)
	return nil
}

func validClientSetting(value string) bool {
	if len(value) > 1024 {
		return false
	}
	for _, char := range value {
		if char < 0x20 || char > 0x7e {
			return false
		}
	}
	return true
}

func clientProfile() *ClientProfile {
	if profile := configuredClientProfile.Load(); profile != nil {
		return profile
	}
	return &defaultClientProfile
}

func (p *ClientProfile) language() string {
	return strings.TrimSpace(strings.SplitN(strings.SplitN(p.AcceptLanguage, ",", 2)[0], ";", 2)[0])
}

func (p *ClientProfile) timezoneOffsetMinutes() int {
	_, offset := time.Now().In(p.location).Zone()
	return -offset / 60
}

func setBrowserRequestHeaders(req *http.Request) {
	profile := clientProfile()
	req.Header.Set("User-Agent", profile.BrowserUserAgent)
	req.Header.Set("Accept-Language", profile.AcceptLanguage)
}

func setWebClientMetadata(req *http.Request) {
	profile := clientProfile()
	req.Header.Set("Oai-Language", profile.language())
	if profile.WebVersion != "" {
		req.Header.Set("Oai-Client-Version", profile.WebVersion)
	}
	if profile.WebBuild != "" {
		req.Header.Set("Oai-Client-Build-Number", profile.WebBuild)
	}
}

func setCodexRequestHeaders(req *http.Request) {
	req.Header.Set("User-Agent", clientProfile().CodexUserAgent)
}
