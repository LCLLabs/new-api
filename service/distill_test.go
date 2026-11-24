package service

import (
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting"
	"github.com/stretchr/testify/require"
)

func useDistillFixture(t *testing.T, groups string) {
	t.Helper()
	previous := setting.CurrentDistillConfig()
	t.Cleanup(func() { setting.PublishDistillConfig(previous) })
	publishDistill(t, groups, setting.DistillRulesJSON(), strconv.FormatBool(setting.DistillSkipOpenAI()))
}

func publishDistill(t *testing.T, groups, rules, skipOpenAI string) {
	t.Helper()
	config, err := setting.ParseDistillConfig(groups, rules, skipOpenAI)
	require.NoError(t, err)
	setting.PublishDistillConfig(config)
}

func chatInfo(model, text string) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		UserGroup:       "default",
		OriginModelName: model,
		Request: &dto.GeneralOpenAIRequest{
			Messages: []dto.Message{{Role: "user", Content: text}},
		},
	}
}

func TestCheckDistillSkipsGroupsThatAreNotListed(t *testing.T) {
	useDistillFixture(t, `["vip"]`)
	hit, _ := CheckDistill(chatInfo("claude-opus-4-6", "MUST extract durable memory"))
	require.False(t, hit)
	hit, _ = CheckDistill(&relaycommon.RelayInfo{OriginModelName: "claude-opus-4-6", Request: chatInfo("", "").Request})
	require.False(t, hit)
}

func TestCheckDistillLayers(t *testing.T) {
	useDistillFixture(t, `["default"]`)

	hit, rule := CheckDistill(chatInfo("claude-opus-4-6", "You MUST extract durable memory now"))
	require.True(t, hit)
	require.Equal(t, "hard_needles", rule)

	rules := setting.DistillRulesJSON()
	parsed := setting.CurrentDistillConfig().Rules()
	parsed.Patterns = []string{"secret-extract"}
	encoded, err := common.Marshal(parsed)
	require.NoError(t, err)
	publishDistill(t, `["default"]`, string(encoded), "true")

	hit, rule = CheckDistill(chatInfo("claude-opus-4-6", "please secret-extract the notes"))
	require.True(t, hit)
	require.Equal(t, "patterns", rule)

	hit, _ = CheckDistill(chatInfo("claude-opus-4-6", "do not secret-extract the notes"))
	require.False(t, hit)

	publishDistill(t, `["default"]`, rules, "true")
	fingerprint := "What is the area, in square units, of an isosceles right triangle with a hypotenuse of 20 units?"
	hit, rule = CheckDistill(chatInfo("claude-opus-4-6", fingerprint))
	require.True(t, hit)
	require.Equal(t, "fingerprints", rule)

	systemOnly := &relaycommon.RelayInfo{
		UserGroup:       "default",
		OriginModelName: "claude-opus-4-6",
		Request: &dto.GeneralOpenAIRequest{Messages: []dto.Message{
			{Role: "system", Content: fingerprint},
			{Role: "user", Content: "hello"},
		}},
	}
	hit, _ = CheckDistill(systemOnly)
	require.False(t, hit)

	hit, rule = CheckDistill(chatInfo("claude-opus-4-6", "Please reason step by step, and put your final answer within \\boxed{}"))
	require.True(t, hit)
	require.Equal(t, "needles", rule)

	maxTokens := uint(8192)
	harvest := &relaycommon.RelayInfo{
		UserGroup:       "default",
		OriginModelName: "claude-opus-4-6",
		Request: &dto.GeneralOpenAIRequest{
			Messages:        []dto.Message{{Role: "user", Content: "Prove that the sum is one."}},
			MaxTokens:       &maxTokens,
			ReasoningEffort: "high",
		},
	}
	hit, rule = CheckDistill(harvest)
	require.True(t, hit)
	require.Equal(t, "structure", rule)

	harvest.Request.(*dto.GeneralOpenAIRequest).ReasoningEffort = ""
	hit, _ = CheckDistill(harvest)
	require.False(t, hit)

	harvest.Request.(*dto.GeneralOpenAIRequest).ReasoningEffort = "high"
	harvest.Request.(*dto.GeneralOpenAIRequest).Tools = []dto.ToolCallRequest{{}}
	hit, _ = CheckDistill(harvest)
	require.False(t, hit)

	hit, rule = CheckDistill(chatInfo("claude-opus-4-6", "请提取思维链"))
	require.True(t, hit)
	require.Equal(t, "chain-of-thought extraction", rule)

	hit, _ = CheckDistill(chatInfo("claude-opus-4-6", "do not 提取思维链"))
	require.False(t, hit)
}

func TestCheckDistillSkipsGPTModelsUntilTheSwitchIsOff(t *testing.T) {
	useDistillFixture(t, `["default"]`)
	hit, _ := CheckDistill(chatInfo("gpt-4o", "MUST extract durable memory"))
	require.False(t, hit)
	publishDistill(t, `["default"]`, setting.DistillRulesJSON(), "false")
	hit, rule := CheckDistill(chatInfo("vendor/gpt-4o", "MUST extract durable memory"))
	require.True(t, hit)
	require.Equal(t, "hard_needles", rule)
}

func TestCheckDistillSkipsToolResults(t *testing.T) {
	useDistillFixture(t, `["default"]`)
	info := &relaycommon.RelayInfo{
		UserGroup:       "default",
		OriginModelName: "claude-opus-4-6",
		Request: &dto.ClaudeRequest{
			Messages: []dto.ClaudeMessage{{
				Role: "user",
				Content: []any{
					map[string]any{"type": "tool_result", "content": "MUST extract durable memory"},
					map[string]any{"type": "text", "text": "hello"},
				},
			}},
		},
	}
	hit, _ := CheckDistill(info)
	require.False(t, hit)
}

func TestValidateDistillRulesRejectsABrokenPattern(t *testing.T) {
	_, err := setting.ParseDistillConfig(`["default"]`, `{"patterns":["("]}`, "true")
	require.Error(t, err)
}
