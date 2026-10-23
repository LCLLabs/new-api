package plugins_test

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	builtinplugins "github.com/QuantumNous/new-api/plugins"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnyLLMResponsesProtocol(t *testing.T) {
	testVideoResponsesProtocol(t, videoResponsesTestCase{
		pluginKey: "anyllm",
		model:     "doubao-seedance-2-0-260128",
		requestBody: map[string]any{
			"model":   "doubao-seedance-2-0-260128",
			"input":   "a cat running through a night city",
			"seconds": 5,
			"size":    "1280x720",
		},
		wantAction: "text_to_video",
		wantRequest: map[string]any{
			"model":   "doubao-seedance-2-0-260128",
			"prompt":  "a cat running through a night city",
			"seconds": float64(5),
			"metadata": map[string]any{
				"resolution": "720p",
			},
		},
		wantUsageKeys:  []string{"duration", "resolution", "video_input"},
		wantVendorName: "anyllm",
	})
}

func TestAnyLLMVideoWireFormat(t *testing.T) {
	source, err := builtinplugins.Source("anyllm")
	require.NoError(t, err)
	registry := jsplugin.NewRegistry()
	plugin, err := registry.RegisterFactory(source, jsplugin.Options{Key: "anyllm"})
	require.NoError(t, err)

	t.Run("submits the documented generation body", func(t *testing.T) {
		value, callErr := plugin.Engine.Call(t.Context(), "buildSubmitRequest", map[string]any{
			"baseUrl": "https://anyllm.cc/api/v1", "apiKey": "sk-test", "upstreamModel": "doubao-seedance-2-5-260628",
			"requestBody": map[string]any{
				"model": "doubao-seedance-2-5-260628", "prompt": "a fox", "seconds": float64(-1),
				"metadata": map[string]any{"resolution": "1080p", "ratio": "16:9"},
			},
		})
		require.NoError(t, callErr)
		body := callObject(t, value)
		assert.Equal(t, "https://anyllm.cc/api/v1/videos/generations", body["url"])
		payload, ok := body["body"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "doubao-seedance-2-5-260628", payload["model"])
		assert.Equal(t, "1080p", payload["resolution"])
		assert.Equal(t, "16:9", payload["ratio"])
		assert.Equal(t, float64(-1), payload["duration"])
		assert.Equal(t, "async", payload["replyType"])
		content, ok := payload["content"].([]any)
		require.True(t, ok)
		require.Len(t, content, 1)
		assert.Equal(t, map[string]any{"type": "text", "text": "a fox"}, content[0])
	})

	t.Run("reads task id and running status", func(t *testing.T) {
		value, callErr := plugin.Engine.Call(t.Context(), "parseSubmitResponse", map[string]any{}, map[string]any{
			"statusCode": float64(200),
			"body":       map[string]any{"id": "vid-1", "status": "running"},
		})
		require.NoError(t, callErr)
		assert.Equal(t, "vid-1", callObject(t, value)["taskId"])

		result, callErr := plugin.Engine.Call(t.Context(), "parseTaskResult", map[string]any{}, map[string]any{"status": "succeeded"}, map[string]any{"status": float64(200)})
		require.NoError(t, callErr)
		assert.Equal(t, "SUCCESS", callObject(t, result)["status"])

		failed, callErr := plugin.Engine.Call(t.Context(), "parseTaskResult", map[string]any{}, map[string]any{
			"status": "failed", "error": map[string]any{"message": "upstream failed", "type": "upstream_error"},
		}, map[string]any{"status": float64(200)})
		require.NoError(t, callErr)
		parsed := callObject(t, failed)
		assert.Equal(t, "FAILURE", parsed["status"])
		assert.Equal(t, "upstream failed", parsed["reason"])
	})
}

func callObject(t *testing.T, value any) map[string]any {
	t.Helper()
	encoded, err := common.Marshal(value)
	require.NoError(t, err)
	var object map[string]any
	require.NoError(t, common.Unmarshal(encoded, &object))
	return object
}
