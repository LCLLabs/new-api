package promptarchive

import (
	"bufio"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArchiveAsyncAppendsSameDayJSONLAndDelta(t *testing.T) {
	dir := t.TempDir()
	prevEnabled := common.PromptArchiveEnabled
	prevDir := common.PromptArchiveDir
	common.PromptArchiveEnabled = true
	common.PromptArchiveDir = dir
	t.Cleanup(func() {
		common.PromptArchiveEnabled = prevEnabled
		common.PromptArchiveDir = prevDir
		ResetLastPathCacheForTest()
	})
	ResetLastPathCacheForTest()

	day := time.Date(2026, 10, 23, 12, 0, 0, 0, time.Local)
	path := ArchivePath(dir, 7, "s1", day)

	req1 := []byte(`{"messages":[{"role":"user","content":"u1"}],"metadata":{"session_id":"s1"}}`)
	resp1 := []byte(`{"choices":[{"message":{"role":"assistant","content":"a1"}}]}`)
	require.NoError(t, ArchiveAsync(BuildInput{
		RequestID:    "req1",
		UserID:       7,
		ModelName:    "gpt-test",
		Path:         "/v1/chat/completions",
		Status:       200,
		CreatedAt:    day,
		RequestBody:  req1,
		ResponseBody: resp1,
		ResponseCT:   "application/json",
	}))
	require.FileExists(t, path)

	req2 := []byte(`{"messages":[{"role":"user","content":"u1"},{"role":"assistant","content":"a1"},{"role":"user","content":"u2"}],"metadata":{"session_id":"s1"}}`)
	resp2 := []byte(`{"choices":[{"message":{"role":"assistant","content":"a2"}}]}`)
	require.NoError(t, ArchiveAsync(BuildInput{
		RequestID:    "req2",
		UserID:       7,
		ModelName:    "gpt-test",
		Path:         "/v1/chat/completions",
		Status:       200,
		CreatedAt:    day.Add(time.Minute),
		RequestBody:  req2,
		ResponseBody: resp2,
		ResponseCT:   "application/json",
	}))

	n, err := CountJSONLLines(path)
	require.NoError(t, err)
	assert.Equal(t, 2, n)

	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()
	scanner := bufio.NewScanner(f)
	var last Record
	for scanner.Scan() {
		require.NoError(t, common.Unmarshal(scanner.Bytes(), &last))
	}
	require.NoError(t, scanner.Err())
	assert.Equal(t, "req2", last.RequestID)
	assert.Equal(t, "req1", last.PrevRequestID)
	assert.Equal(t, "u2", last.UserTurn)
	delta, ok := last.MessagesDelta.([]any)
	require.True(t, ok)
	require.Len(t, delta, 2)
}

func TestArchiveAsyncStreamSSE(t *testing.T) {
	dir := t.TempDir()
	prevDir := common.PromptArchiveDir
	common.PromptArchiveDir = dir
	t.Cleanup(func() {
		common.PromptArchiveDir = prevDir
		ResetLastPathCacheForTest()
	})
	ResetLastPathCacheForTest()

	now := time.Now()
	sse := "data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\n"
	require.NoError(t, ArchiveAsync(BuildInput{
		RequestID:    "stream1",
		UserID:       1,
		Path:         "/v1/chat/completions",
		Status:       200,
		CreatedAt:    now,
		RequestBody:  []byte(`{"messages":[{"role":"user","content":"x"}]}`),
		ResponseBody: []byte(sse),
		ResponseCT:   "text/event-stream",
	}))
	path := ArchivePath(dir, 1, NoSessionID, now)
	require.FileExists(t, path)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var rec Record
	require.NoError(t, common.Unmarshal(data, &rec))
	assert.True(t, rec.IsStream)
	assert.Equal(t, sse, rec.ResponseSSE)
	assert.Nil(t, rec.ResponseContent)
}

func TestNosessionSameDaySingleFile(t *testing.T) {
	dir := t.TempDir()
	prevDir := common.PromptArchiveDir
	common.PromptArchiveDir = dir
	t.Cleanup(func() {
		common.PromptArchiveDir = prevDir
		ResetLastPathCacheForTest()
	})
	ResetLastPathCacheForTest()

	day := time.Date(2026, 10, 25, 10, 0, 0, 0, time.Local)
	for i, id := range []string{"a", "b", "c"} {
		require.NoError(t, ArchiveAsync(BuildInput{
			RequestID:    id,
			UserID:       1,
			Path:         "/v1/chat/completions",
			Status:       200,
			CreatedAt:    day.Add(time.Duration(i) * time.Minute),
			RequestBody:  []byte(`{"messages":[{"role":"user","content":"x"}]}`),
			ResponseBody: []byte(`{"ok":true}`),
			ResponseCT:   "application/json",
		}))
	}
	path := ArchivePath(dir, 1, NoSessionID, day)
	n, err := CountJSONLLines(path)
	require.NoError(t, err)
	assert.Equal(t, 3, n)
	matches, err := filepath.Glob(filepath.Join(dir, "by-user", "1", "nosession", "*.jsonl"))
	require.NoError(t, err)
	assert.Len(t, matches, 1)
}
