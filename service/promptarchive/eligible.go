package promptarchive

import "strings"

// PathEligible reports whether the request path should be archived.
func PathEligible(path string) bool {
	if path == "" {
		return false
	}
	switch path {
	case "/v1/chat/completions", "/v1/completions", "/v1/messages",
		"/v1/responses", "/v1/responses/compact",
		"/pg/chat/completions":
		return true
	}
	if strings.Contains(path, ":generateContent") || strings.Contains(path, ":streamGenerateContent") {
		return true
	}
	if strings.HasPrefix(path, "/v1/models/") {
		lower := strings.ToLower(path)
		if strings.Contains(lower, "embed") {
			return false
		}
		return true
	}
	return false
}
