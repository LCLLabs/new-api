package obsmetrics

import (
	"crypto/subtle"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	outcomeSuccess       = "success"
	outcomeRateLimit     = "rate_limit"
	outcomeTimeout       = "timeout"
	outcomeAuth          = "auth"
	outcomeUpstreamError = "upstream_error"
)

var latencyBuckets = []float64{0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10, 30, 60, 120, 300}

// RelayObservation is one finished relay. Ignored samples follow the same
// health classification as performance metrics: client cancellation and
// business rejection are not latency samples.
type RelayObservation struct {
	Ignored       bool
	Stream        bool
	HasFirstToken bool
	HasClock      bool
	FirstToken    time.Duration
	Duration      time.Duration
	ChannelID     int
	OutputTokens  int64
	InputTokens   int64
}

type instruments struct {
	ttft        *prometheus.HistogramVec
	nonstream   *prometheus.HistogramVec
	throughput  *prometheus.HistogramVec
	tokens      *prometheus.CounterVec
	inputTokens *prometheus.CounterVec
	upstream    *prometheus.CounterVec
	channels    *prometheus.GaugeVec
	responses   *prometheus.CounterVec
	handler     http.Handler
}

var (
	mu     sync.RWMutex
	active *instruments
)

// Enable registers the process metrics on the default Prometheus registry.
// Calling it more than once is a no-op.
func Enable() {
	mu.Lock()
	defer mu.Unlock()
	if active != nil {
		return
	}
	active = newInstruments(prometheus.DefaultRegisterer, prometheus.DefaultGatherer)
}

func newInstruments(reg prometheus.Registerer, gatherer prometheus.Gatherer) *instruments {
	inst := &instruments{
		ttft: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "newapi_relay_ttft_seconds",
			Help:    "Time to first token for streaming relay requests that sent a first token.",
			Buckets: latencyBuckets,
		}, []string{"channel"}),
		nonstream: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "newapi_relay_nonstream_duration_seconds",
			Help:    "End-to-end duration of non-streaming relay requests.",
			Buckets: latencyBuckets,
		}, []string{"channel"}),
		throughput: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "newapi_relay_output_tokens_per_second",
			Help:    "Output tokens per second for one finished streaming response.",
			Buckets: []float64{5, 10, 20, 40, 80, 120, 200, 400, 800},
		}, []string{"channel"}),
		tokens: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "newapi_relay_output_tokens_total",
			Help: "Output tokens from finished streaming responses, attributed to the completion minute.",
		}, []string{"channel"}),
		inputTokens: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "newapi_relay_input_tokens_total",
			Help: "Prompt tokens from finished relay requests, attributed to the completion minute.",
		}, []string{"channel"}),
		upstream: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "newapi_relay_upstream_outcomes_total",
			Help: "Upstream attempts by channel. Failures are counted per attempt and success once per finished relay.",
		}, []string{"channel", "outcome"}),
		channels: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "newapi_channel_info",
			Help: "Channel id and its current display name.",
		}, []string{"channel", "channel_name"}),
		responses: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "newapi_http_responses_total",
			Help: "HTTP responses by status code and route tag.",
		}, []string{"status", "route"}),
		handler: promhttp.HandlerFor(gatherer, promhttp.HandlerOpts{}),
	}
	reg.MustRegister(inst.ttft, inst.nonstream, inst.throughput, inst.tokens, inst.inputTokens, inst.upstream, inst.channels, inst.responses)
	return inst
}

// ObserveRelay records streaming time-to-first-token or non-streaming
// end-to-end duration. It does nothing until Enable has been called.
func ObserveRelay(obs RelayObservation) {
	mu.RLock()
	inst := active
	mu.RUnlock()
	if inst == nil {
		return
	}
	inst.observeRelay(obs)
}

func (inst *instruments) observeRelay(obs RelayObservation) {
	channel, ok := channelLabel(obs.ChannelID)
	if !ok || obs.Ignored {
		return
	}
	if obs.InputTokens > 0 {
		inst.inputTokens.WithLabelValues(channel).Add(float64(obs.InputTokens))
	}
	if obs.Stream {
		if obs.HasFirstToken && obs.FirstToken >= 0 {
			inst.ttft.WithLabelValues(channel).Observe(obs.FirstToken.Seconds())
		}
		inst.observeStreamThroughput(channel, obs)
		return
	}
	if obs.HasClock && obs.Duration >= 0 {
		inst.nonstream.WithLabelValues(channel).Observe(obs.Duration.Seconds())
	}
}

func (inst *instruments) observeStreamThroughput(channel string, obs RelayObservation) {
	if !obs.HasFirstToken || obs.OutputTokens <= 0 {
		return
	}
	inst.tokens.WithLabelValues(channel).Add(float64(obs.OutputTokens))
	generation := obs.Duration - obs.FirstToken
	if obs.HasClock && generation > 0 {
		inst.throughput.WithLabelValues(channel).Observe(float64(obs.OutputTokens) / generation.Seconds())
	}
}

// ObserveUpstreamSuccess counts one finished relay on the channel that completed it.
func ObserveUpstreamSuccess(channelID int) {
	mu.RLock()
	inst := active
	mu.RUnlock()
	channel, ok := channelLabel(channelID)
	if inst == nil || !ok {
		return
	}
	inst.upstream.WithLabelValues(channel, outcomeSuccess).Inc()
}

// ObserveUpstreamFailure counts one failed upstream attempt. Any status outside
// 2xx is a failure. skip drops client cancellation and business rejection.
// Channel id 0 is not recorded.
func ObserveUpstreamFailure(channelID, statusCode int, timedOut, skip bool) {
	mu.RLock()
	inst := active
	mu.RUnlock()
	channel, ok := channelLabel(channelID)
	if inst == nil || !ok || skip {
		return
	}
	outcome := upstreamFailureOutcome(statusCode, timedOut)
	if outcome == "" {
		return
	}
	inst.upstream.WithLabelValues(channel, outcome).Inc()
}

func upstreamFailureOutcome(statusCode int, timedOut bool) string {
	switch {
	case timedOut || statusCode == http.StatusRequestTimeout || statusCode == http.StatusGatewayTimeout:
		return outcomeTimeout
	case statusCode == http.StatusTooManyRequests:
		return outcomeRateLimit
	case statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden:
		return outcomeAuth
	case statusCode >= 200 && statusCode < 300:
		return ""
	default:
		return outcomeUpstreamError
	}
}

func channelLabel(channelID int) (string, bool) {
	if channelID <= 0 {
		return "", false
	}
	return strconv.Itoa(channelID), true
}

// SetChannelNames publishes the current channel display names. The channel id
// remains the series key on relay metrics; this gauge only supplies the name.
func SetChannelNames(names map[int]string) {
	mu.RLock()
	inst := active
	mu.RUnlock()
	if inst == nil {
		return
	}
	inst.channels.Reset()
	for id, name := range names {
		label, ok := channelLabel(id)
		if !ok {
			continue
		}
		inst.channels.WithLabelValues(label, channelDisplayName(label, name)).Set(1)
	}
}

func channelDisplayName(label, name string) string {
	name = strings.Join(strings.Fields(name), " ")
	name = strings.ReplaceAll(name, `"`, "'")
	if name == "" {
		return label
	}
	return name
}

// ObserveHTTP counts one finished HTTP response. An empty route tag is
// recorded as web, matching the access log default. It does nothing until
// Enable has been called.
func ObserveHTTP(status int, route string) {
	mu.RLock()
	inst := active
	mu.RUnlock()
	if inst == nil {
		return
	}
	if route == "" {
		route = "web"
	}
	if status == 0 {
		status = http.StatusOK
	}
	inst.responses.WithLabelValues(strconv.Itoa(status), route).Inc()
}

// Handler serves Prometheus text exposition after a Bearer token check.
func Handler(token string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !bearerAuthorized(c.GetHeader("Authorization"), token) {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		mu.RLock()
		inst := active
		mu.RUnlock()
		if inst == nil {
			c.AbortWithStatus(http.StatusServiceUnavailable)
			return
		}
		inst.handler.ServeHTTP(c.Writer, c.Request)
	}
}

func bearerAuthorized(header, token string) bool {
	scheme, value, ok := strings.Cut(header, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") || token == "" || len(value) != len(token) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(value), []byte(token)) == 1
}
