package relayconvert

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/relayconvert/convmeta"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertRequestResponsesToClaudeFlattensTopLevelOneOf(t *testing.T) {
	params, err := os.ReadFile("internal/shared/claude/testdata/automation_update_params.json")
	require.NoError(t, err)

	var before map[string]any
	require.NoError(t, json.Unmarshal(params, &before))
	require.Contains(t, before, "oneOf")

	tools, err := json.Marshal([]map[string]any{{
		"type":        "function",
		"name":        "mcp__codex_app__automation_update",
		"description": "Create, update, view, or delete recurring automations",
		"parameters":   json.RawMessage(params),
	}})
	require.NoError(t, err)

	maxTokens := uint(128)
	req := &dto.OpenAIResponsesRequest{
		Model:           "claude-opus-5-5",
		MaxOutputTokens: &maxTokens,
		Input:           json.RawMessage(`[{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}]`),
		Tools:           tools,
	}

	result, err := ConvertRequest(nil, &convmeta.Values{}, types.RelayFormatClaude, req)
	require.NoError(t, err)
	claudeReq, ok := result.Value.(*dto.ClaudeRequest)
	require.True(t, ok)

	toolsSlice, ok := claudeReq.Tools.([]any)
	require.True(t, ok)
	require.Len(t, toolsSlice, 1)

	toolJSON, err := json.Marshal(toolsSlice[0])
	require.NoError(t, err)
	var tool map[string]any
	require.NoError(t, json.Unmarshal(toolJSON, &tool))
	assert.Equal(t, "mcp__codex_app__automation_update", tool["name"])

	inputSchema, ok := tool["input_schema"].(map[string]any)
	require.True(t, ok)
	assert.NotContains(t, inputSchema, "oneOf")
	assert.NotContains(t, inputSchema, "anyOf")
	assert.NotContains(t, inputSchema, "allOf")
	assert.Equal(t, "object", inputSchema["type"])
	props, ok := inputSchema["properties"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, props, "mode")
	assert.Contains(t, props, "id")
}
