package common

import (
	"errors"
	"net/url"
	"strings"
)

// ValidateRootRedirectURL accepts absolute http(s) destinations used for the
// optional root-path redirect. Empty values are rejected here; callers that
// allow clearing the option must check emptiness themselves.
func ValidateRootRedirectURL(raw string) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return errors.New("root redirect URL is empty")
	}
	if raw != trimmed {
		return errors.New("root redirect URL must not contain surrounding whitespace")
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed == nil {
		return errors.New("root redirect URL is invalid")
	}
	if !strings.EqualFold(parsed.Scheme, "http") && !strings.EqualFold(parsed.Scheme, "https") {
		return errors.New("root redirect URL must use http or https")
	}
	if parsed.Host == "" || parsed.User != nil || parsed.Opaque != "" {
		return errors.New("root redirect URL must contain a host and no userinfo")
	}
	return nil
}
