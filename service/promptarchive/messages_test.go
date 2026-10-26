package promptarchive

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMessagesDeltaStrictPrefix(t *testing.T) {
	t.Parallel()
	prev := []any{
		map[string]any{"role": "user", "content": "u1"},
	}
	curr := []any{
		map[string]any{"role": "user", "content": "u1"},
		map[string]any{"role": "assistant", "content": "a1"},
		map[string]any{"role": "user", "content": "u2"},
	}
	delta, ok := MessagesDelta(prev, curr)
	require.True(t, ok)
	require.Len(t, delta, 2)
	assert.Equal(t, "u2", UserTurn(curr))

	_, ok = MessagesDelta(curr, prev)
	assert.False(t, ok)

	_, ok = MessagesDelta(prev, []any{
		map[string]any{"role": "user", "content": "other"},
		map[string]any{"role": "user", "content": "u2"},
	})
	assert.False(t, ok)
}

func TestExtractMessagesFormats(t *testing.T) {
	t.Parallel()
	chat := map[string]any{"messages": []any{map[string]any{"role": "user", "content": "a"}}}
	assert.Len(t, ExtractMessages(chat), 1)

	gemini := map[string]any{"contents": []any{map[string]any{"role": "user", "parts": []any{map[string]any{"text": "g"}}}}}
	msgs := ExtractMessages(gemini)
	assert.Equal(t, "g", UserTurn(msgs))

	resp := map[string]any{"input": "hello"}
	assert.Equal(t, "hello", UserTurn(ExtractMessages(resp)))
}
