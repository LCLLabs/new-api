package promptarchive

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPathEligible(t *testing.T) {
	t.Parallel()
	assert.True(t, PathEligible("/v1/chat/completions"))
	assert.True(t, PathEligible("/v1/completions"))
	assert.True(t, PathEligible("/v1/messages"))
	assert.True(t, PathEligible("/v1/responses"))
	assert.True(t, PathEligible("/v1/responses/compact"))
	assert.True(t, PathEligible("/pg/chat/completions"))
	assert.True(t, PathEligible("/v1beta/models/gemini-2.0-flash:generateContent"))
	assert.True(t, PathEligible("/v1/models/gemini-pro:streamGenerateContent"))
	assert.False(t, PathEligible("/v1/embeddings"))
	assert.False(t, PathEligible("/v1/images/generations"))
	assert.False(t, PathEligible("/v1/models/text-embedding-004:embedContent"))
	assert.False(t, PathEligible("/v1/audio/speech"))
}
