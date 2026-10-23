package obsmetrics_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/pkg/obsmetrics"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestObserveRelaySeparatesStreamTTFTAndNonStreamDuration(t *testing.T) {
	registry := obsmetrics.InstallRegistry(t)
	start := time.Now().Add(-2 * time.Second)

	perfmetrics.RecordRelayResult(context.Background(), &relaycommon.RelayInfo{
		ChannelMeta:       &relaycommon.ChannelMeta{ChannelId: 3},
		IsStream:          true,
		StartTime:         start,
		FirstResponseTime: start.Add(400 * time.Millisecond),
	}, nil)

	ttftCount, ttftSum := histogramSample(t, registry, "newapi_relay_ttft_seconds", "3")
	assert.Equal(t, uint64(1), ttftCount)
	assert.InDelta(t, 0.4, ttftSum, 0.000001)
	nonStreamCount, _ := histogramSample(t, registry, "newapi_relay_nonstream_duration_seconds", "3")
	assert.Equal(t, uint64(0), nonStreamCount)

	perfmetrics.RecordRelayResult(context.Background(), &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 3},
		StartTime:   start,
	}, nil)

	ttftCount, _ = histogramSample(t, registry, "newapi_relay_ttft_seconds", "3")
	assert.Equal(t, uint64(1), ttftCount)
	nonStreamCount, nonStreamSum := histogramSample(t, registry, "newapi_relay_nonstream_duration_seconds", "3")
	assert.Equal(t, uint64(1), nonStreamCount)
	assert.InDelta(t, 2, nonStreamSum, 0.5)

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	perfmetrics.RecordRelayResult(canceled, &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 3},
		StartTime:   start,
	}, nil)
	nonStreamCount, _ = histogramSample(t, registry, "newapi_relay_nonstream_duration_seconds", "3")
	assert.Equal(t, uint64(2), nonStreamCount)
}

func TestObserveRelaySkipsIgnoredAndStreamWithoutFirstToken(t *testing.T) {
	registry := obsmetrics.InstallRegistry(t)
	start := time.Now().Add(-time.Second)

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	perfmetrics.RecordRelayResult(canceled, &relaycommon.RelayInfo{
		ChannelMeta:       &relaycommon.ChannelMeta{ChannelId: 7},
		IsStream:          true,
		StartTime:         start,
		FirstResponseTime: start.Add(100 * time.Millisecond),
	}, nil)
	perfmetrics.RecordRelayResult(context.Background(), &relaycommon.RelayInfo{
		ChannelMeta:                  &relaycommon.ChannelMeta{ChannelId: 7},
		IsStream:                     true,
		StartTime:                    start,
		PerformanceBusinessRejection: true,
	}, nil)
	perfmetrics.RecordRelayResult(context.Background(), &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 8},
		IsStream:    true,
		StartTime:   start,
	}, nil)

	ttftCount, _ := histogramSample(t, registry, "newapi_relay_ttft_seconds", "7")
	nonStreamCount, _ := histogramSample(t, registry, "newapi_relay_nonstream_duration_seconds", "7")
	assert.Equal(t, uint64(0), ttftCount)
	assert.Equal(t, uint64(0), nonStreamCount)
	assert.Equal(t, 0.0, labeledCounter(t, registry, "newapi_relay_upstream_outcomes_total", map[string]string{"channel": "7", "outcome": "success"}))
}

func TestObserveHTTPCountsStatusAndRoute(t *testing.T) {
	registry := obsmetrics.InstallRegistry(t)

	obsmetrics.ObserveHTTP(http.StatusServiceUnavailable, "relay")
	obsmetrics.ObserveHTTP(http.StatusOK, "")

	assert.Equal(t, 1.0, counterValue(t, registry, "503", "relay"))
	assert.Equal(t, 1.0, counterValue(t, registry, "200", "web"))
	assert.Equal(t, 0.0, counterValue(t, registry, "503", "web"))
}

func TestMetricsHandlerRequiresBearerToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	obsmetrics.InstallRegistry(t)
	start := time.Now().Add(-time.Second)
	perfmetrics.RecordRelayResult(context.Background(), &relaycommon.RelayInfo{
		ChannelMeta:       &relaycommon.ChannelMeta{ChannelId: 1},
		IsStream:          true,
		StartTime:         start,
		FirstResponseTime: start.Add(100 * time.Millisecond),
	}, nil)
	perfmetrics.RecordRelayResult(context.Background(), &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 1},
		StartTime:   start,
	}, nil)
	router := gin.New()
	router.GET("/metrics", obsmetrics.Handler("secret-token"))

	unauthorized := performMetricsRequest(router, "")
	require.Equal(t, http.StatusUnauthorized, unauthorized.Code)
	assert.NotContains(t, unauthorized.Body.String(), "newapi_relay_ttft_seconds")

	wrong := performMetricsRequest(router, "Bearer secret-tokex")
	require.Equal(t, http.StatusUnauthorized, wrong.Code)

	differentLength := performMetricsRequest(router, "Bearer no")
	require.Equal(t, http.StatusUnauthorized, differentLength.Code)

	authorized := performMetricsRequest(router, "Bearer secret-token")
	require.Equal(t, http.StatusOK, authorized.Code)
	assert.Contains(t, authorized.Body.String(), "newapi_relay_ttft_seconds")
	assert.Contains(t, authorized.Body.String(), "newapi_relay_nonstream_duration_seconds")
}

func performMetricsRequest(router *gin.Engine, authorization string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	if authorization != "" {
		request.Header.Set("Authorization", authorization)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func TestRelayChannelThroughputAndOutcomes(t *testing.T) {
	registry := obsmetrics.InstallRegistry(t)
	start := time.Now().Add(-2 * time.Second)

	perfmetrics.RecordRelayResult(context.Background(), &relaycommon.RelayInfo{
		ChannelMeta:             &relaycommon.ChannelMeta{ChannelId: 0},
		IsStream:                true,
		StartTime:               start,
		FirstResponseTime:       start.Add(400 * time.Millisecond),
		PerformanceOutputTokens: 160,
	}, nil)
	assert.Equal(t, uint64(0), mustHistogramCount(t, registry, "newapi_relay_ttft_seconds", ""))

	perfmetrics.RecordRelayResult(context.Background(), &relaycommon.RelayInfo{
		ChannelMeta:             &relaycommon.ChannelMeta{ChannelId: 11},
		IsStream:                true,
		StartTime:               start,
		FirstResponseTime:       start.Add(400 * time.Millisecond),
		PerformanceOutputTokens: 160,
		PerformanceInputTokens:  40,
	}, nil)
	count, sum := histogramSample(t, registry, "newapi_relay_output_tokens_per_second", "11")
	assert.Equal(t, uint64(1), count)
	assert.InDelta(t, 100, sum, 15)
	assert.Equal(t, 160.0, labeledCounter(t, registry, "newapi_relay_output_tokens_total", map[string]string{"channel": "11"}))
	assert.Equal(t, 40.0, labeledCounter(t, registry, "newapi_relay_input_tokens_total", map[string]string{"channel": "11"}))
	assert.Equal(t, 1.0, labeledCounter(t, registry, "newapi_relay_upstream_outcomes_total", map[string]string{"channel": "11", "outcome": "success"}))

	perfmetrics.RecordRelayResult(context.Background(), &relaycommon.RelayInfo{
		ChannelMeta:             &relaycommon.ChannelMeta{ChannelId: 11},
		IsStream:                true,
		StartTime:               start,
		FirstResponseTime:       start.Add(400 * time.Millisecond),
		PerformanceOutputTokens: 0,
	}, nil)
	count, _ = histogramSample(t, registry, "newapi_relay_output_tokens_per_second", "11")
	assert.Equal(t, uint64(1), count)
	assert.Equal(t, 160.0, labeledCounter(t, registry, "newapi_relay_output_tokens_total", map[string]string{"channel": "11"}))

	perfmetrics.RecordRelayResult(context.Background(), &relaycommon.RelayInfo{
		ChannelMeta:             &relaycommon.ChannelMeta{ChannelId: 11},
		IsStream:                true,
		StartTime:               start,
		PerformanceOutputTokens: 80,
	}, nil)
	count, _ = histogramSample(t, registry, "newapi_relay_output_tokens_per_second", "11")
	assert.Equal(t, uint64(1), count)
	assert.Equal(t, 160.0, labeledCounter(t, registry, "newapi_relay_output_tokens_total", map[string]string{"channel": "11"}))

	perfmetrics.RecordRelayResult(context.Background(), &relaycommon.RelayInfo{
		ChannelMeta:             &relaycommon.ChannelMeta{ChannelId: 11},
		StartTime:               start,
		PerformanceOutputTokens: 80,
		PerformanceInputTokens:  25,
	}, nil)
	assert.Equal(t, 160.0, labeledCounter(t, registry, "newapi_relay_output_tokens_total", map[string]string{"channel": "11"}))
	assert.Equal(t, 65.0, labeledCounter(t, registry, "newapi_relay_input_tokens_total", map[string]string{"channel": "11"}))

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	perfmetrics.RecordRelayResult(canceled, &relaycommon.RelayInfo{
		ChannelMeta:             &relaycommon.ChannelMeta{ChannelId: 12},
		IsStream:                true,
		StartTime:               start,
		FirstResponseTime:       start.Add(100 * time.Millisecond),
		PerformanceOutputTokens: 40,
		PerformanceInputTokens:  30,
	}, nil)
	perfmetrics.RecordRelayResult(context.Background(), &relaycommon.RelayInfo{
		ChannelMeta:                  &relaycommon.ChannelMeta{ChannelId: 12},
		IsStream:                     true,
		StartTime:                    start,
		FirstResponseTime:            start.Add(100 * time.Millisecond),
		PerformanceOutputTokens:      40,
		PerformanceBusinessRejection: true,
	}, nil)
	assert.Equal(t, 0.0, labeledCounter(t, registry, "newapi_relay_upstream_outcomes_total", map[string]string{"channel": "12", "outcome": "success"}))
	assert.Equal(t, 0.0, labeledCounter(t, registry, "newapi_relay_output_tokens_total", map[string]string{"channel": "12"}))
	assert.Equal(t, 0.0, labeledCounter(t, registry, "newapi_relay_input_tokens_total", map[string]string{"channel": "12"}))

	perfmetrics.RecordRelayResult(context.Background(), &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 4},
		StartTime:   start,
	}, types.NewOpenAIError(errors.New("upstream"), types.ErrorCodeBadResponseStatusCode, http.StatusBadGateway))
	assert.Equal(t, 0.0, labeledCounter(t, registry, "newapi_relay_upstream_outcomes_total", map[string]string{"channel": "4", "outcome": "success"}))
	assert.Equal(t, 0.0, labeledCounter(t, registry, "newapi_relay_upstream_outcomes_total", map[string]string{"channel": "4", "outcome": "upstream_error"}))

	obsmetrics.ObserveUpstreamFailure(4, http.StatusTooManyRequests, false, false)
	obsmetrics.ObserveUpstreamFailure(4, http.StatusInternalServerError, false, false)
	obsmetrics.ObserveUpstreamFailure(4, http.StatusGatewayTimeout, false, false)
	obsmetrics.ObserveUpstreamFailure(5, http.StatusInternalServerError, true, false)
	obsmetrics.ObserveUpstreamFailure(4, http.StatusBadRequest, false, false)
	obsmetrics.ObserveUpstreamFailure(9, http.StatusUnauthorized, false, false)
	obsmetrics.ObserveUpstreamFailure(9, http.StatusForbidden, false, false)
	obsmetrics.ObserveUpstreamFailure(9, 0, false, false)
	obsmetrics.ObserveUpstreamFailure(9, http.StatusOK, false, false)
	obsmetrics.ObserveUpstreamFailure(0, http.StatusInternalServerError, false, false)
	obsmetrics.ObserveUpstreamFailure(4, http.StatusInternalServerError, false, true)
	assert.Equal(t, 1.0, labeledCounter(t, registry, "newapi_relay_upstream_outcomes_total", map[string]string{"channel": "4", "outcome": "rate_limit"}))
	assert.Equal(t, 2.0, labeledCounter(t, registry, "newapi_relay_upstream_outcomes_total", map[string]string{"channel": "4", "outcome": "upstream_error"}))
	assert.Equal(t, 1.0, labeledCounter(t, registry, "newapi_relay_upstream_outcomes_total", map[string]string{"channel": "4", "outcome": "timeout"}))
	assert.Equal(t, 1.0, labeledCounter(t, registry, "newapi_relay_upstream_outcomes_total", map[string]string{"channel": "5", "outcome": "timeout"}))
	assert.Equal(t, 2.0, labeledCounter(t, registry, "newapi_relay_upstream_outcomes_total", map[string]string{"channel": "9", "outcome": "auth"}))
	assert.Equal(t, 1.0, labeledCounter(t, registry, "newapi_relay_upstream_outcomes_total", map[string]string{"channel": "9", "outcome": "upstream_error"}))
	assert.Equal(t, 0.0, labeledCounter(t, registry, "newapi_relay_upstream_outcomes_total", map[string]string{"channel": "4", "outcome": "auth"}))
	assert.Equal(t, 0.0, labeledCounter(t, registry, "newapi_relay_upstream_outcomes_total", map[string]string{"channel": "5", "outcome": "upstream_error"}))
	assert.Equal(t, 0.0, labeledCounter(t, registry, "newapi_relay_upstream_outcomes_total", map[string]string{"channel": "4", "outcome": "success"}))
	assert.Equal(t, 0.0, labeledCounter(t, registry, "newapi_relay_upstream_outcomes_total", map[string]string{"channel": "9", "outcome": "success"}))
}

func TestSetChannelNamesPublishesDisplayName(t *testing.T) {
	registry := obsmetrics.InstallRegistry(t)

	obsmetrics.SetChannelNames(map[int]string{
		0:  "ignored",
		3:  "  openai \"primary\"  ",
		11: "",
	})

	assert.Equal(t, 1.0, labeledGauge(t, registry, "newapi_channel_info", map[string]string{"channel": "3", "channel_name": "openai 'primary'"}))
	assert.Equal(t, 1.0, labeledGauge(t, registry, "newapi_channel_info", map[string]string{"channel": "11", "channel_name": "11"}))
	assert.Equal(t, 0.0, labeledGauge(t, registry, "newapi_channel_info", map[string]string{"channel": "0", "channel_name": "ignored"}))

	obsmetrics.SetChannelNames(map[int]string{3: "renamed"})
	assert.Equal(t, 1.0, labeledGauge(t, registry, "newapi_channel_info", map[string]string{"channel": "3", "channel_name": "renamed"}))
	assert.Equal(t, 0.0, labeledGauge(t, registry, "newapi_channel_info", map[string]string{"channel": "11", "channel_name": "11"}))
}

func histogramSample(t *testing.T, gatherer prometheus.Gatherer, name, channel string) (uint64, float64) {
	t.Helper()
	family := findFamily(t, gatherer, name)
	if family == nil {
		return 0, 0
	}
	var count uint64
	var sum float64
	for _, metric := range family.GetMetric() {
		if channel != "" && metricLabels(metric)["channel"] != channel {
			continue
		}
		count += metric.GetHistogram().GetSampleCount()
		sum += metric.GetHistogram().GetSampleSum()
	}
	return count, sum
}

func mustHistogramCount(t *testing.T, gatherer prometheus.Gatherer, name, channel string) uint64 {
	t.Helper()
	count, _ := histogramSample(t, gatherer, name, channel)
	return count
}

func counterValue(t *testing.T, gatherer prometheus.Gatherer, status, route string) float64 {
	t.Helper()
	return labeledCounter(t, gatherer, "newapi_http_responses_total", map[string]string{"status": status, "route": route})
}

func labeledCounter(t *testing.T, gatherer prometheus.Gatherer, name string, want map[string]string) float64 {
	t.Helper()
	return labeledValue(t, gatherer, name, want, func(metric *dto.Metric) float64 {
		return metric.GetCounter().GetValue()
	})
}

func labeledGauge(t *testing.T, gatherer prometheus.Gatherer, name string, want map[string]string) float64 {
	t.Helper()
	return labeledValue(t, gatherer, name, want, func(metric *dto.Metric) float64 {
		return metric.GetGauge().GetValue()
	})
}

func labeledValue(t *testing.T, gatherer prometheus.Gatherer, name string, want map[string]string, value func(*dto.Metric) float64) float64 {
	t.Helper()
	family := findFamily(t, gatherer, name)
	if family == nil {
		return 0
	}
	for _, metric := range family.GetMetric() {
		labels := metricLabels(metric)
		matched := true
		for key, wantValue := range want {
			if labels[key] != wantValue {
				matched = false
				break
			}
		}
		if matched {
			return value(metric)
		}
	}
	return 0
}

func metricLabels(metric *dto.Metric) map[string]string {
	labels := map[string]string{}
	for _, label := range metric.GetLabel() {
		labels[label.GetName()] = label.GetValue()
	}
	return labels
}

func findFamily(t *testing.T, gatherer prometheus.Gatherer, name string) *dto.MetricFamily {
	t.Helper()
	families, err := gatherer.Gather()
	require.NoError(t, err)
	for _, family := range families {
		if family.GetName() == name {
			return family
		}
	}
	return nil
}
