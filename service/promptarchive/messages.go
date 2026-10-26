package promptarchive

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// ExtractMessages returns a comparable message list from common text request shapes.
func ExtractMessages(requestObj any) []any {
	m, ok := requestObj.(map[string]any)
	if !ok || m == nil {
		return nil
	}
	if msgs, ok := asArray(m["messages"]); ok {
		return msgs
	}
	if contents, ok := asArray(m["contents"]); ok {
		return contents
	}
	if input, exists := m["input"]; exists {
		switch v := input.(type) {
		case string:
			if strings.TrimSpace(v) == "" {
				return nil
			}
			return []any{map[string]any{"role": "user", "content": v}}
		case []any:
			return v
		}
	}
	return nil
}

func asArray(v any) ([]any, bool) {
	arr, ok := v.([]any)
	return arr, ok && arr != nil
}

// UserTurn extracts the last user-visible text turn from a message list.
func UserTurn(messages []any) string {
	for i := len(messages) - 1; i >= 0; i-- {
		msg, ok := messages[i].(map[string]any)
		if !ok {
			continue
		}
		role := strings.ToLower(fmt.Sprint(msg["role"]))
		if role == "" {
			// Gemini contents use "role" similarly; also accept user parts without role at end.
			if parts, ok := asArray(msg["parts"]); ok {
				if text := joinPartsText(parts); text != "" {
					return text
				}
			}
			continue
		}
		if role != "user" && role != "human" {
			continue
		}
		if text := messageText(msg); text != "" {
			return text
		}
	}
	return ""
}

func messageText(msg map[string]any) string {
	if parts, ok := asArray(msg["parts"]); ok {
		return joinPartsText(parts)
	}
	switch c := msg["content"].(type) {
	case string:
		return c
	case []any:
		var b strings.Builder
		for _, item := range c {
			switch t := item.(type) {
			case string:
				b.WriteString(t)
			case map[string]any:
				if s, ok := t["text"].(string); ok {
					b.WriteString(s)
				}
			}
		}
		return b.String()
	case map[string]any:
		if s, ok := c["text"].(string); ok {
			return s
		}
	}
	return ""
}

func joinPartsText(parts []any) string {
	var b strings.Builder
	for _, p := range parts {
		m, ok := p.(map[string]any)
		if !ok {
			continue
		}
		if s, ok := m["text"].(string); ok {
			b.WriteString(s)
		}
	}
	return b.String()
}

// MessagesDelta returns the suffix of curr when prev is a strict prefix; otherwise nil and false.
func MessagesDelta(prev, curr []any) (delta []any, ok bool) {
	if len(prev) == 0 || len(curr) <= len(prev) {
		return nil, false
	}
	for i := range prev {
		if !messageEqual(prev[i], curr[i]) {
			return nil, false
		}
	}
	delta = make([]any, len(curr)-len(prev))
	copy(delta, curr[len(prev):])
	return delta, true
}

func messageEqual(a, b any) bool {
	ab, err := common.Marshal(a)
	if err != nil {
		return false
	}
	bb, err := common.Marshal(b)
	if err != nil {
		return false
	}
	return string(ab) == string(bb)
}
