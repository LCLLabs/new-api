package promptarchive

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/tidwall/gjson"
)

const NoSessionID = "nosession"

// ExtractSessionID picks a session identifier from headers then request JSON.
func ExtractSessionID(headerSession, headerConversation string, requestJSON []byte) string {
	if v := sanitizeSessionSegment(headerSession); v != "" {
		return v
	}
	if v := sanitizeSessionSegment(headerConversation); v != "" {
		return v
	}
	if len(requestJSON) == 0 {
		return NoSessionID
	}
	for _, path := range []string{
		"metadata.session_id",
		"metadata.conversation_id",
		"prompt_cache_key",
	} {
		if v := sanitizeSessionSegment(gjson.GetBytes(requestJSON, path).String()); v != "" {
			return v
		}
	}
	conv := gjson.GetBytes(requestJSON, "conversation")
	switch {
	case conv.Type == gjson.String:
		if v := sanitizeSessionSegment(conv.String()); v != "" {
			return v
		}
	case conv.IsObject():
		if v := sanitizeSessionSegment(conv.Get("id").String()); v != "" {
			return v
		}
		if raw := strings.TrimSpace(conv.Raw); raw != "" && raw != "null" {
			if v := sanitizeSessionSegment(raw); v != "" {
				return v
			}
		}
	}
	return NoSessionID
}

func sanitizeSessionSegment(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(value))
	for _, r := range value {
		switch r {
		case '/', '\\', 0:
			b.WriteByte('_')
		default:
			if r < 32 {
				b.WriteByte('_')
				continue
			}
			b.WriteRune(r)
		}
	}
	out := b.String()
	if out == "." || out == ".." {
		return "_"
	}
	if len(out) > 200 {
		out = out[:200]
	}
	return out
}

// CopyBytesCap copies src up to max bytes and reports truncation.
func CopyBytesCap(src []byte, max int) (out []byte, truncated bool) {
	if max <= 0 {
		max = common.PromptArchiveMaxRequestBytes
	}
	if len(src) <= max {
		out = make([]byte, len(src))
		copy(out, src)
		return out, false
	}
	out = make([]byte, max)
	copy(out, src[:max])
	return out, true
}
