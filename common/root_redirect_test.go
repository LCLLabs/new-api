package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateRootRedirectURL(t *testing.T) {
	t.Parallel()

	for _, valid := range []string{
		"https://basamp.com/",
		"http://localhost:3000",
		"https://example.com/path?x=1",
	} {
		require.NoError(t, ValidateRootRedirectURL(valid), valid)
	}

	for _, invalid := range []string{
		"",
		" ",
		"/relative",
		"ftp://example.com",
		"javascript:alert(1)",
		"https://user:secret@example.com",
		" https://example.com",
		"https://example.com ",
	} {
		assert.Error(t, ValidateRootRedirectURL(invalid), invalid)
	}
}
