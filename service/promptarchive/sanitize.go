package promptarchive

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/QuantumNous/new-api/common"
)

const binaryOmitPrefix = "[omitted binary len="
const maxKeepStringLen = 512

// SanitizeJSONValue recursively redacts large binary / data-URL strings.
func SanitizeJSONValue(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, child := range t {
			t[k] = SanitizeJSONValue(child)
		}
		return t
	case []any:
		for i, child := range t {
			t[i] = SanitizeJSONValue(child)
		}
		return t
	case string:
		return sanitizeString(t)
	default:
		return v
	}
}

func sanitizeString(s string) string {
	if strings.HasPrefix(s, "data:") {
		if idx := strings.Index(s, ","); idx >= 0 && idx+1 < len(s) {
			payload := s[idx+1:]
			return binaryOmitPrefix + fmt.Sprintf("%d]", len(payload))
		}
		return binaryOmitPrefix + fmt.Sprintf("%d]", len(s))
	}
	if len(s) > maxKeepStringLen && looksLikeBase64(s) {
		return binaryOmitPrefix + fmt.Sprintf("%d]", len(s))
	}
	return s
}

func looksLikeBase64(s string) bool {
	if len(s) < maxKeepStringLen {
		return false
	}
	sample := s
	if len(sample) > 4096 {
		sample = sample[:4096]
	}
	ok := 0
	for _, r := range sample {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') ||
			r == '+' || r == '/' || r == '=' || r == '-' || r == '_' || unicode.IsSpace(r) {
			ok++
			continue
		}
		return false
	}
	return ok > 0
}

// ParseAndSanitizeRequest unmarshals request JSON, sanitizes, and remarshals for storage fields.
func ParseAndSanitizeRequest(raw []byte) (requestObj any, messages []any, err error) {
	if len(raw) == 0 {
		return map[string]any{}, nil, nil
	}
	var obj any
	if err := common.Unmarshal(raw, &obj); err != nil {
		// Keep a truncated raw string view when JSON is incomplete due to byte cap.
		return map[string]any{"_raw": string(raw), "_parse_error": err.Error()}, nil, nil
	}
	obj = SanitizeJSONValue(obj)
	messages = ExtractMessages(obj)
	return obj, messages, nil
}
