package promptarchive

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractSessionIDPriority(t *testing.T) {
	t.Parallel()
	body := []byte(`{"metadata":{"session_id":"from-meta"},"prompt_cache_key":"from-cache","conversation":"from-conv"}`)
	assert.Equal(t, "header-session", ExtractSessionID("header-session", "header-conv", body))
	assert.Equal(t, "header-conv", ExtractSessionID("", "header-conv", body))
	assert.Equal(t, "from-meta", ExtractSessionID("", "", body))
	assert.Equal(t, "from-cache", ExtractSessionID("", "", []byte(`{"prompt_cache_key":"from-cache","conversation":"from-conv"}`)))
	assert.Equal(t, "from-conv", ExtractSessionID("", "", []byte(`{"conversation":"from-conv"}`)))
	assert.Equal(t, "conv-obj", ExtractSessionID("", "", []byte(`{"conversation":{"id":"conv-obj"}}`)))
	assert.Equal(t, NoSessionID, ExtractSessionID("", "", []byte(`{"model":"x"}`)))
}

func TestSanitizeSessionSegment(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "a_b", sanitizeSessionSegment("a/b"))
	assert.Equal(t, "_", sanitizeSessionSegment(".."))
}

func TestCopyBytesCap(t *testing.T) {
	t.Parallel()
	out, trunc := CopyBytesCap([]byte("hello"), 10)
	require.False(t, trunc)
	assert.Equal(t, "hello", string(out))
	out, trunc = CopyBytesCap([]byte("hello world"), 5)
	require.True(t, trunc)
	assert.Equal(t, "hello", string(out))
}
