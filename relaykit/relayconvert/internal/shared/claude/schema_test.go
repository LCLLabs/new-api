package claude

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFunctionParametersToInputSchemaFlattensTopLevelOneOf(t *testing.T) {
	schema, ok := FunctionParametersToInputSchema(json.RawMessage(`{
		"type":"object",
		"properties":{},
		"oneOf":[
			{"type":"object","properties":{"mode":{"type":"string","enum":["view"]},"id":{"type":"string"}},"required":["mode","id"]},
			{"type":"object","properties":{"mode":{"type":"string","enum":["delete"]},"id":{"type":"string"}},"required":["mode","id"]}
		]
	}`))
	require.True(t, ok)

	var got map[string]any
	require.NoError(t, json.Unmarshal(schema, &got))
	assert.NotContains(t, got, "oneOf")
	assert.Equal(t, "object", got["type"])
	props := got["properties"].(map[string]any)
	assert.Contains(t, props, "mode")
	assert.Contains(t, props, "id")
	mode := props["mode"].(map[string]any)
	assert.Contains(t, mode, "anyOf")
	required := asStringSet(got["required"])
	assert.Equal(t, map[string]struct{}{"mode": {}, "id": {}}, required)
}

func TestFunctionParametersToInputSchemaKeepsNestedOneOf(t *testing.T) {
	schema, ok := FunctionParametersToInputSchema(json.RawMessage(`{
		"type":"object",
		"properties":{
			"target":{"oneOf":[{"type":"object","properties":{"a":{"type":"string"}}},{"type":"object","properties":{"b":{"type":"string"}}}]}
		}
	}`))
	require.True(t, ok)
	var got map[string]any
	require.NoError(t, json.Unmarshal(schema, &got))
	assert.NotContains(t, got, "oneOf")
	target := got["properties"].(map[string]any)["target"].(map[string]any)
	assert.Contains(t, target, "oneOf")
}

func TestFunctionParametersToInputSchemaFlattensAutomationUpdateArchive(t *testing.T) {
	params, err := os.ReadFile("testdata/automation_update_params.json")
	require.NoError(t, err)
	schema, ok := FunctionParametersToInputSchema(params)
	require.True(t, ok)
	var got map[string]any
	require.NoError(t, json.Unmarshal(schema, &got))
	assert.NotContains(t, got, "oneOf")
	assert.NotContains(t, got, "anyOf")
	assert.NotContains(t, got, "allOf")
	assert.Equal(t, "object", got["type"])
	props := got["properties"].(map[string]any)
	assert.NotEmpty(t, props)
	assert.Contains(t, props, "mode")
	assert.Contains(t, props, "id")
	assert.Contains(t, got, "$defs")
}

func TestFunctionParametersToInputSchemaAllOfUnionsRequired(t *testing.T) {
	schema, ok := FunctionParametersToInputSchema(json.RawMessage(`{
		"allOf":[
			{"type":"object","properties":{"a":{"type":"string"}},"required":["a"]},
			{"type":"object","properties":{"b":{"type":"number"}},"required":["b"]}
		]
	}`))
	require.True(t, ok)
	var got map[string]any
	require.NoError(t, json.Unmarshal(schema, &got))
	assert.NotContains(t, got, "allOf")
	props := got["properties"].(map[string]any)
	assert.Contains(t, props, "a")
	assert.Contains(t, props, "b")
	assert.Equal(t, map[string]struct{}{"a": {}, "b": {}}, asStringSet(got["required"]))
}

func asStringSet(raw any) map[string]struct{} {
	out := map[string]struct{}{}
	list, ok := raw.([]any)
	if !ok {
		return out
	}
	for _, item := range list {
		if s, ok := item.(string); ok {
			out[s] = struct{}{}
		}
	}
	return out
}
