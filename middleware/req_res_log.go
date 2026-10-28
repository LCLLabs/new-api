package middleware

import (
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

const reqResLogPrefix = "[REQ-RES] "
const finalRequestErrorKey = "final_request_error"

// RememberFinalRequestError stores the error reason that will be returned to the
// client for this request. Later calls overwrite earlier ones so only the
// final failure reason is kept for the merged [REQ-RES] line.
func RememberFinalRequestError(c *gin.Context, message string) {
	if c == nil || message == "" {
		return
	}
	c.Set(finalRequestErrorKey, message)
}

func appendReqResLog(line string, param gin.LogFormatterParams) string {
	tag, _ := param.Keys[RouteTagKey].(string)
	if tag != "relay" {
		return line
	}
	requestID, _ := param.Keys[common.RequestIdKey].(string)
	path := param.Path
	if strings.HasPrefix(path, "/api/oauth/") || strings.HasPrefix(path, "/oauth/") {
		path, _, _ = strings.Cut(path, "?")
	}
	end := param.TimeStamp
	start := requestStartTime(requestID, end.Add(-param.Latency))
	record := map[string]any{
		"start_time": start.In(time.Local).Format(time.RFC3339),
		"end_time":   end.In(time.Local).Format(time.RFC3339),
		"node":       common.NodeName,
		"status":     strconv.Itoa(param.StatusCode),
		"method":     param.Method,
		"client_ip":  filterLabel(param.ClientIP),
		"request_id": requestID,
		"path":       path,
		"latency":    param.Latency.String(),
		"user_id":    "-",
		"token_id":   "-",
		"token_name": "-",
		"channel_id": "-",
		"model_name": "-",
		"group":      "-",
		"is_stream":  "-",
	}
	if consume, ok := model.RequestConsumeLogFrom(param.Keys); ok {
		record["user_id"] = strconv.Itoa(consume.UserID)
		record["token_id"] = strconv.Itoa(consume.Params.TokenId)
		record["token_name"] = filterLabel(consume.Params.TokenName)
		record["channel_id"] = strconv.Itoa(consume.Params.ChannelId)
		record["model_name"] = filterLabel(consume.Params.ModelName)
		record["group"] = filterLabel(consume.Params.Group)
		record["is_stream"] = strconv.FormatBool(consume.Params.IsStream)
		record["prompt_tokens"] = consume.Params.PromptTokens
		record["completion_tokens"] = consume.Params.CompletionTokens
		record["quota"] = consume.Params.Quota
		record["content"] = consume.Params.Content
		record["use_time_seconds"] = consume.Params.UseTimeSeconds
		mergeConsumeOther(record, consume.Other)
	}
	if msg, ok := param.Keys[finalRequestErrorKey].(string); ok && msg != "" {
		record["error"] = msg
	}
	encoded, err := common.Marshal(record)
	if err != nil {
		return line
	}
	return line + reqResLogPrefix + string(encoded) + "\n"
}

func filterLabel(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

func requestStartTime(requestID string, fallback time.Time) time.Time {
	if len(requestID) >= 14 {
		if parsed, err := time.ParseInLocation("20060102150405", requestID[:14], time.UTC); err == nil {
			return parsed
		}
	}
	return fallback
}

func mergeConsumeOther(record map[string]any, other map[string]any) {
	if len(other) == 0 {
		return
	}
	if admin, ok := other["admin_info"].(map[string]any); ok {
		for key, value := range admin {
			record[key] = value
		}
	}
	for key, value := range other {
		if key == "admin_info" {
			continue
		}
		record[key] = value
	}
}
