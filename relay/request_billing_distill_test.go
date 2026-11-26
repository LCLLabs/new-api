package relay

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDistillBlockedEmitsUserEvent(t *testing.T) {
	previous := setting.CurrentDistillConfig()
	t.Cleanup(func() { setting.PublishDistillConfig(previous) })
	config, err := setting.ParseDistillConfig(`["default"]`, setting.DistillRulesJSON(), "false")
	require.NoError(t, err)
	setting.PublishDistillConfig(config)

	var logs bytes.Buffer
	common.LogWriterMu.Lock()
	oldWriter := gin.DefaultErrorWriter
	gin.DefaultErrorWriter = &logs
	common.LogWriterMu.Unlock()
	t.Cleanup(func() {
		common.LogWriterMu.Lock()
		gin.DefaultErrorWriter = oldWriter
		common.LogWriterMu.Unlock()
	})

	const prompt = "MUST extract durable memory PRIVATE_PROMPT"
	requests := []struct {
		name    string
		request dto.Request
	}{
		{"chat", &dto.GeneralOpenAIRequest{Messages: []dto.Message{{Role: "user", Content: prompt}}}},
		{"claude", &dto.ClaudeRequest{Messages: []dto.ClaudeMessage{{Role: "user", Content: prompt}}}},
		{"responses", &dto.OpenAIResponsesRequest{Input: []byte(`"MUST extract durable memory PRIVATE_PROMPT"`)}},
	}
	eventIDs := make(map[string]struct{})
	for _, tc := range requests {
		t.Run(tc.name, func(t *testing.T) {
			logs.Reset()
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
			c.Set(common.RequestIdKey, "shared-request-id")
			info := &relaycommon.RelayInfo{
				UserId:          123,
				UserGroup:       "default",
				OriginModelName: "claude-example",
				RequestId:       "shared-request-id",
				Request:         tc.request,
			}

			apiErr := PrepareRequestBilling(c, info)
			require.NotNil(t, apiErr)
			assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
			assert.Equal(t, types.ErrorCodeDistillBlocked, apiErr.GetErrorCode())
			assert.Nil(t, info.Billing, "blocked requests must not create a billing session")

			output := logs.String()
			assert.Equal(t, 1, strings.Count(strings.TrimSpace(output), "\n")+1, "one blocked request should produce one warning")
			require.Equal(t, 1, strings.Count(output, "DISTILL_EVENT "))
			_, payload, ok := strings.Cut(output, "DISTILL_EVENT ")
			require.True(t, ok)
			var event struct {
				Event     string `json:"event"`
				UserID    int    `json:"user_id"`
				Rule      string `json:"rule"`
				RequestID string `json:"request_id"`
				EventID   string `json:"event_id"`
			}
			require.NoError(t, common.UnmarshalJsonStr(strings.TrimSpace(payload), &event))
			assert.Equal(t, "distill_blocked", event.Event)
			assert.Equal(t, 123, event.UserID)
			assert.Equal(t, "hard_needles", event.Rule)
			assert.Equal(t, "shared-request-id", event.RequestID)
			require.NotEmpty(t, event.EventID)
			assert.NotContains(t, eventIDs, event.EventID, "events sharing a request ID need distinct event IDs")
			eventIDs[event.EventID] = struct{}{}
			assert.NotContains(t, output, "PRIVATE_PROMPT")
		})
	}
}
