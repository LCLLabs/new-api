package promptarchive

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSanitizeJSONValueOmitsDataURLAndBase64(t *testing.T) {
	t.Parallel()
	longB64 := strings.Repeat("A", 600)
	in := map[string]any{
		"messages": []any{
			map[string]any{
				"role":    "user",
				"content": "data:image/png;base64,AAAA",
			},
			map[string]any{
				"role":    "user",
				"content": longB64,
			},
		},
	}
	out := SanitizeJSONValue(in).(map[string]any)
	msgs := out["messages"].([]any)
	c0 := msgs[0].(map[string]any)["content"].(string)
	c1 := msgs[1].(map[string]any)["content"].(string)
	assert.Contains(t, c0, "[omitted binary len=")
	assert.Contains(t, c1, "[omitted binary len=")
}

func TestParseAndSanitizeRequest(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"messages":[{"role":"user","content":"hi"}],"model":"gpt"}`)
	obj, msgs, err := ParseAndSanitizeRequest(raw)
	require.NoError(t, err)
	require.NotNil(t, obj)
	require.Len(t, msgs, 1)
	assert.Equal(t, "hi", UserTurn(msgs))
}
