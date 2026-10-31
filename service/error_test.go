package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestResetStatusCode(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name             string
		statusCode       int
		statusCodeConfig string
		expectedCode     int
	}{
		{
			name:             "map string value",
			statusCode:       429,
			statusCodeConfig: `{"429":"503"}`,
			expectedCode:     503,
		},
		{
			name:             "map int value",
			statusCode:       429,
			statusCodeConfig: `{"429":503}`,
			expectedCode:     503,
		},
		{
			name:             "skip invalid string value",
			statusCode:       429,
			statusCodeConfig: `{"429":"bad-code"}`,
			expectedCode:     429,
		},
		{
			name:             "skip status code 200",
			statusCode:       200,
			statusCodeConfig: `{"200":503}`,
			expectedCode:     200,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			newAPIError := &types.NewAPIError{
				StatusCode: tc.statusCode,
			}
			ResetStatusCode(newAPIError, tc.statusCodeConfig)
			require.Equal(t, tc.expectedCode, newAPIError.StatusCode)
		})
	}
}

func TestMapUpstreamErrorForClient(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		err        *types.NewAPIError
		wantStatus int
		prepare    func(*types.NewAPIError)
	}{
		{
			name:       "upstream 400 becomes 502",
			err:        types.NewOpenAIError(fmt.Errorf("bad request"), types.ErrorCodeBadResponseStatusCode, http.StatusBadRequest),
			wantStatus: http.StatusBadGateway,
		},
		{
			name:       "upstream 401 becomes 502",
			err:        types.NewOpenAIError(fmt.Errorf("invalid api key"), types.ErrorCodeBadResponseStatusCode, http.StatusUnauthorized),
			wantStatus: http.StatusBadGateway,
		},
		{
			name:       "upstream 403 becomes 502",
			err:        types.NewOpenAIError(fmt.Errorf("forbidden"), types.ErrorCodeBadResponseStatusCode, http.StatusForbidden),
			wantStatus: http.StatusBadGateway,
		},
		{
			name:       "upstream 422 becomes 502",
			err:        types.NewOpenAIError(fmt.Errorf("unprocessable"), types.ErrorCodeBadResponseStatusCode, http.StatusUnprocessableEntity),
			wantStatus: http.StatusBadGateway,
		},
		{
			name:       "upstream 429 becomes 503",
			err:        types.NewOpenAIError(fmt.Errorf("rate limited"), types.ErrorCodeBadResponseStatusCode, http.StatusTooManyRequests),
			wantStatus: http.StatusServiceUnavailable,
		},
		{
			name:       "upstream 408 becomes 504",
			err:        types.NewOpenAIError(fmt.Errorf("timeout"), types.ErrorCodeBadResponseStatusCode, http.StatusRequestTimeout),
			wantStatus: http.StatusGatewayTimeout,
		},
		{
			name:       "upstream 504 stays 504",
			err:        types.NewOpenAIError(fmt.Errorf("gateway timeout"), types.ErrorCodeBadResponseStatusCode, http.StatusGatewayTimeout),
			wantStatus: http.StatusGatewayTimeout,
		},
		{
			name:       "upstream 500 stays 500",
			err:        types.NewOpenAIError(fmt.Errorf("server error"), types.ErrorCodeBadResponseStatusCode, http.StatusInternalServerError),
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "channel invalid key becomes 502",
			err:        types.NewErrorWithStatusCode(fmt.Errorf("bad key"), types.ErrorCodeChannelInvalidKey, http.StatusUnauthorized),
			wantStatus: http.StatusBadGateway,
		},
		{
			name:       "do request failed with zero status becomes 502",
			err:        types.NewErrorWithStatusCode(fmt.Errorf("dial tcp"), types.ErrorCodeDoRequestFailed, 0),
			wantStatus: http.StatusBadGateway,
		},
		{
			name:       "local invalid request stays 400",
			err:        types.NewError(fmt.Errorf("missing model"), types.ErrorCodeInvalidRequest, types.ErrOptionWithStatusCode(http.StatusBadRequest), types.ErrOptionWithSkipRetry()),
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "local insufficient quota stays 403",
			err:        types.NewErrorWithStatusCode(fmt.Errorf("quota"), types.ErrorCodeInsufficientUserQuota, http.StatusForbidden),
			wantStatus: http.StatusForbidden,
		},
		{
			name: "status_code_mapping to 4xx still lifted to 502",
			err:  types.NewOpenAIError(fmt.Errorf("upstream auth"), types.ErrorCodeBadResponseStatusCode, http.StatusUnauthorized),
			prepare: func(err *types.NewAPIError) {
				ResetStatusCode(err, `{"401":"400"}`)
			},
			wantStatus: http.StatusBadGateway,
		},
		{
			name: "status_code_mapping to 5xx kept",
			err:  types.NewOpenAIError(fmt.Errorf("upstream auth"), types.ErrorCodeBadResponseStatusCode, http.StatusUnauthorized),
			prepare: func(err *types.NewAPIError) {
				ResetStatusCode(err, `{"401":"502"}`)
			},
			wantStatus: http.StatusBadGateway,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.prepare != nil {
				tc.prepare(tc.err)
			}
			MapUpstreamErrorForClient(tc.err)
			require.Equal(t, tc.wantStatus, tc.err.StatusCode)
		})
	}
}

func TestMapUpstreamTaskErrorForClient(t *testing.T) {
	t.Parallel()

	upstream := TaskErrorWrapper(fmt.Errorf("invalid credential"), "upstream_auth", http.StatusUnauthorized)
	MapUpstreamTaskErrorForClient(upstream)
	require.Equal(t, http.StatusBadGateway, upstream.StatusCode)

	local := TaskErrorWrapperLocal(fmt.Errorf("bad params"), "invalid_request", http.StatusBadRequest)
	MapUpstreamTaskErrorForClient(local)
	require.Equal(t, http.StatusBadRequest, local.StatusCode)
}

func TestRelayErrorHandlerTruncatesInvalidJSONBodyInLog(t *testing.T) {
	withDebugEnabled(t, false)

	body := strings.Repeat("b", common.LocalLogContentLimit+256)
	var logBuffer bytes.Buffer

	common.LogWriterMu.Lock()
	oldWriter := gin.DefaultErrorWriter
	gin.DefaultErrorWriter = &logBuffer
	common.LogWriterMu.Unlock()
	t.Cleanup(func() {
		common.LogWriterMu.Lock()
		gin.DefaultErrorWriter = oldWriter
		common.LogWriterMu.Unlock()
	})

	resp := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	newAPIError := RelayErrorHandler(context.Background(), resp, false)

	require.NotNil(t, newAPIError)
	require.Equal(t, "bad response status code 500", newAPIError.Error())
	require.Contains(t, logBuffer.String(), "[truncated")
	require.Contains(t, logBuffer.String(), fmt.Sprintf("original_length=%d", len(body)))
	require.NotContains(t, logBuffer.String(), strings.Repeat("b", common.LocalLogContentLimit+1))
}

func TestRelayErrorHandlerKeepsStructuredErrorMessage(t *testing.T) {
	message := strings.Repeat("c", common.LocalLogContentLimit+256)
	body := `{"message":"` + message + `"}`
	resp := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	newAPIError := RelayErrorHandler(context.Background(), resp, false)

	require.NotNil(t, newAPIError)
	require.Equal(t, message, newAPIError.Error())
}

func TestRelayErrorHandlerKeepsOpenAIErrorMessage(t *testing.T) {
	message := strings.Repeat("d", common.LocalLogContentLimit+256)
	body := `{"error":{"message":"` + message + `","type":"server_error","code":"server_error"}}`
	resp := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	newAPIError := RelayErrorHandler(context.Background(), resp, false)

	require.NotNil(t, newAPIError)
	require.Equal(t, message, newAPIError.Error())
}

func TestRelayErrorHandlerKeepsInvalidJSONBodyInDebugLog(t *testing.T) {
	withDebugEnabled(t, true)

	body := strings.Repeat("e", common.LocalLogContentLimit+256)
	var logBuffer bytes.Buffer

	common.LogWriterMu.Lock()
	oldWriter := gin.DefaultErrorWriter
	gin.DefaultErrorWriter = &logBuffer
	common.LogWriterMu.Unlock()
	t.Cleanup(func() {
		common.LogWriterMu.Lock()
		gin.DefaultErrorWriter = oldWriter
		common.LogWriterMu.Unlock()
	})

	resp := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	newAPIError := RelayErrorHandler(context.Background(), resp, false)

	require.NotNil(t, newAPIError)
	require.NotContains(t, logBuffer.String(), "[truncated")
	require.Contains(t, logBuffer.String(), body)
}

func withDebugEnabled(t *testing.T, enabled bool) {
	t.Helper()

	oldDebug := common.DebugEnabled
	common.DebugEnabled = enabled
	t.Cleanup(func() {
		common.DebugEnabled = oldDebug
	})
}
