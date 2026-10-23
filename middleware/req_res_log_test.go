package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRelayRequestWritesReqResLog(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousWriter := gin.DefaultWriter
	previousNode := common.NodeName
	var output bytes.Buffer
	gin.DefaultWriter = &output
	common.NodeName = "n1-10.10.10.198"
	t.Cleanup(func() {
		gin.DefaultWriter = previousWriter
		common.NodeName = previousNode
	})

	other := model.NewLogOther()
	require.True(t, other.SetPublic("request_path", "/v1/chat/completions"))
	require.True(t, other.SetPublic("billing_source", "wallet"))
	require.True(t, other.SetAdmin("use_channel", []string{"3"}))

	router := gin.New()
	SetUpLogger(router)
	router.POST("/v1/chat/completions", RouteTag("relay"), func(c *gin.Context) {
		c.Set(common.RequestIdKey, "202610221226525022079228268d9d6zJYy1KOF")
		model.RememberRequestConsumeLog(c, 2, model.RecordConsumeLogParams{
			ChannelId:        3,
			PromptTokens:     17,
			CompletionTokens: 1001,
			ModelName:        "glm-5.3-flash",
			TokenName:        "xusenqi-default",
			Quota:            193,
			TokenId:          6,
			UseTimeSeconds:   16,
			IsStream:         false,
			Group:            "default",
			Other:            other,
		})
		c.Status(http.StatusOK)
	})

	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	request.RemoteAddr = "117.50.190.4:1234"
	router.ServeHTTP(httptest.NewRecorder(), request)

	body := output.String()
	assert.Contains(t, body, "[GIN]")
	line, ok := reqResLine(body)
	require.True(t, ok)
	var record map[string]any
	require.NoError(t, json.Unmarshal([]byte(line), &record))
	assert.Equal(t, time.Date(2026, 10, 22, 12, 26, 52, 0, time.UTC).In(time.Local).Format(time.RFC3339), record["start_time"])
	assert.Equal(t, "n1-10.10.10.198", record["node"])
	assert.Equal(t, "200", record["status"])
	assert.Equal(t, "POST", record["method"])
	assert.Equal(t, "117.50.190.4", record["client_ip"])
	assert.Equal(t, "2", record["user_id"])
	assert.Equal(t, "3", record["channel_id"])
	assert.Equal(t, "glm-5.3-flash", record["model_name"])
	assert.Equal(t, "6", record["token_id"])
	assert.Equal(t, "xusenqi-default", record["token_name"])
	assert.Equal(t, "default", record["group"])
	assert.Equal(t, "false", record["is_stream"])
	assert.Equal(t, "/v1/chat/completions", record["path"])
	assert.Equal(t, float64(17), record["prompt_tokens"])
	assert.Equal(t, float64(193), record["quota"])
	assert.Equal(t, "wallet", record["billing_source"])
	assert.Equal(t, []any{"3"}, record["use_channel"])
	assert.NotContains(t, record, "admin_info")
}

func TestNonRelayRequestOmitsReqResLog(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousWriter := gin.DefaultWriter
	var output bytes.Buffer
	gin.DefaultWriter = &output
	t.Cleanup(func() { gin.DefaultWriter = previousWriter })

	router := gin.New()
	SetUpLogger(router)
	router.GET("/api/status", RouteTag("api"), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/status", nil))

	assert.NotContains(t, output.String(), "[REQ-RES]")
}

func TestRelayWithoutConsumeLogStillWritesAccessFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousWriter := gin.DefaultWriter
	var output bytes.Buffer
	gin.DefaultWriter = &output
	t.Cleanup(func() { gin.DefaultWriter = previousWriter })

	router := gin.New()
	SetUpLogger(router)
	router.POST("/v1/chat/completions", RouteTag("relay"), func(c *gin.Context) {
		c.Status(http.StatusUnauthorized)
	})
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil))

	line, ok := reqResLine(output.String())
	require.True(t, ok)
	var record map[string]any
	require.NoError(t, json.Unmarshal([]byte(line), &record))
	assert.Equal(t, "401", record["status"])
	assert.Equal(t, "-", record["user_id"])
	assert.Equal(t, "-", record["model_name"])
	assert.NotContains(t, record, "quota")
}

func reqResLine(output string) (string, bool) {
	for line := range strings.SplitSeq(output, "\n") {
		payload, ok := strings.CutPrefix(line, "[REQ-RES] ")
		if ok {
			return payload, true
		}
	}
	return "", false
}
