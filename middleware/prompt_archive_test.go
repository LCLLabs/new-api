package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/service/promptarchive"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPromptArchiveMiddlewareDisabledNoOp(t *testing.T) {
	gin.SetMode(gin.TestMode)
	prev := common.PromptArchiveEnabled
	common.PromptArchiveEnabled = false
	t.Cleanup(func() { common.PromptArchiveEnabled = prev })

	r := gin.New()
	r.Use(PromptArchive())
	r.POST("/v1/chat/completions", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{"messages":[]}`))
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestPromptArchiveMiddlewareWritesJSONLAndSSE(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	prevEnabled := common.PromptArchiveEnabled
	prevDir := common.PromptArchiveDir
	common.PromptArchiveEnabled = true
	common.PromptArchiveDir = dir
	t.Cleanup(func() {
		common.PromptArchiveEnabled = prevEnabled
		common.PromptArchiveDir = prevDir
		promptarchive.ResetLastPathCacheForTest()
	})
	promptarchive.ResetLastPathCacheForTest()

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(common.RequestIdKey, "mid-req-1")
		c.Set("id", 42)
		common.SetContextKey(c, constant.ContextKeyOriginalModel, "gpt-test")
		c.Next()
	})
	r.Use(PromptArchive())
	r.POST("/v1/chat/completions", func(c *gin.Context) {
		body := []byte(`{"messages":[{"role":"user","content":"hello"}],"metadata":{"session_id":"sess-a"}}`)
		storage, err := common.CreateBodyStorage(body)
		require.NoError(t, err)
		c.Set(common.KeyBodyStorage, storage)
		c.JSON(http.StatusOK, gin.H{
			"choices": []gin.H{{"message": gin.H{"role": "assistant", "content": "world"}}},
		})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{}`))
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var dayFile string
	require.Eventually(t, func() bool {
		matches, _ := filepath.Glob(filepath.Join(dir, "by-user", "42", "sess-a", "*.jsonl"))
		if len(matches) != 1 {
			return false
		}
		dayFile = matches[0]
		n, err := promptarchive.CountJSONLLines(dayFile)
		return err == nil && n == 1
	}, 2*time.Second, 20*time.Millisecond)

	data, err := os.ReadFile(dayFile)
	require.NoError(t, err)
	var rec promptarchive.Record
	require.NoError(t, json.Unmarshal(bytes.TrimSpace(data), &rec))
	assert.Equal(t, "hello", rec.UserTurn)
	assert.False(t, rec.IsStream)
	require.NotNil(t, rec.ResponseContent)

	r2 := gin.New()
	r2.Use(func(c *gin.Context) {
		c.Set(common.RequestIdKey, "mid-req-2")
		c.Set("id", 42)
		c.Next()
	})
	r2.Use(PromptArchive())
	r2.POST("/v1/chat/completions", func(c *gin.Context) {
		body := []byte(`{"messages":[{"role":"user","content":"hello"},{"role":"assistant","content":"world"},{"role":"user","content":"again"}],"metadata":{"session_id":"sess-a"}}`)
		storage, err := common.CreateBodyStorage(body)
		require.NoError(t, err)
		c.Set(common.KeyBodyStorage, storage)
		c.Header("Content-Type", "text/event-stream")
		c.Status(http.StatusOK)
		_, _ = c.Writer.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"x\"}}]}\n\n"))
	})
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{}`))
	r2.ServeHTTP(w2, req2)
	require.Equal(t, http.StatusOK, w2.Code)

	require.Eventually(t, func() bool {
		n, err := promptarchive.CountJSONLLines(dayFile)
		if err != nil || n != 2 {
			return false
		}
		raw, err := os.ReadFile(dayFile)
		if err != nil {
			return false
		}
		lines := bytes.Split(bytes.TrimSpace(raw), []byte("\n"))
		if len(lines) < 2 {
			return false
		}
		var rec2 promptarchive.Record
		if err := json.Unmarshal(lines[len(lines)-1], &rec2); err != nil {
			return false
		}
		return rec2.IsStream && rec2.PrevRequestID == "mid-req-1" && rec2.UserTurn == "again"
	}, 2*time.Second, 20*time.Millisecond)
}

func TestPromptArchiveSkipsIneligiblePath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	prevEnabled := common.PromptArchiveEnabled
	prevDir := common.PromptArchiveDir
	common.PromptArchiveEnabled = true
	common.PromptArchiveDir = dir
	t.Cleanup(func() {
		common.PromptArchiveEnabled = prevEnabled
		common.PromptArchiveDir = prevDir
	})

	r := gin.New()
	r.Use(PromptArchive())
	r.POST("/v1/embeddings", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/embeddings", bytes.NewBufferString(`{}`))
	r.ServeHTTP(w, req)
	time.Sleep(50 * time.Millisecond)
	matches, err := filepath.Glob(filepath.Join(dir, "by-user", "*", "*", "*.jsonl"))
	require.NoError(t, err)
	assert.Empty(t, matches)
}
