package promptarchive

import (
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

// TruncationFlags records whether request/response bodies were capped.
type TruncationFlags struct {
	Request  bool `json:"request"`
	Response bool `json:"response"`
}

// Record is one JSONL line in the daily archive file.
type Record struct {
	RequestID       string          `json:"request_id"`
	UserID          int             `json:"user_id"`
	TokenID         int             `json:"token_id"`
	ChannelID       int             `json:"channel_id"`
	ModelName       string          `json:"model_name"`
	Group           string          `json:"group"`
	SessionID       string          `json:"session_id"`
	Path            string          `json:"path"`
	IsStream        bool            `json:"is_stream"`
	Status          int             `json:"status"`
	CreatedAt       string          `json:"created_at"`
	Node            string          `json:"node"`
	RequestJSON     any             `json:"request_json"`
	UserTurn        string          `json:"user_turn"`
	MessagesDelta   any             `json:"messages_delta"`
	PrevRequestID   string          `json:"prev_request_id"`
	ResponseContent any             `json:"response_content,omitempty"`
	ResponseSSE     string          `json:"response_sse,omitempty"`
	Truncated       TruncationFlags `json:"truncated"`
}

// BuildInput holds buffers and metadata collected on the request path.
type BuildInput struct {
	RequestID     string
	UserID        int
	TokenID       int
	ChannelID     int
	ModelName     string
	Group         string
	Path          string
	Status        int
	CreatedAt     time.Time
	Node          string
	HeaderSession string
	HeaderConv    string
	RequestBody   []byte
	RequestTrunc  bool
	ResponseBody  []byte
	ResponseTrunc bool
	ResponseCT    string
}

// BuildRecord sanitizes payloads, computes session delta, and returns the record plus archive path.
func BuildRecord(in BuildInput) (Record, string, []any, error) {
	at := in.CreatedAt
	if at.IsZero() {
		at = time.Now()
	}
	reqObj, messages, _ := ParseAndSanitizeRequest(in.RequestBody)
	sessionID := ExtractSessionID(in.HeaderSession, in.HeaderConv, in.RequestBody)
	if sessionID == NoSessionID {
		if m, ok := reqObj.(map[string]any); ok {
			if raw, err := common.Marshal(m); err == nil {
				sessionID = ExtractSessionID(in.HeaderSession, in.HeaderConv, raw)
			}
		}
	}

	path := ArchivePath(common.PromptArchiveDir, in.UserID, sessionID, at)

	rec := Record{
		RequestID:   in.RequestID,
		UserID:      in.UserID,
		TokenID:     in.TokenID,
		ChannelID:   in.ChannelID,
		ModelName:   in.ModelName,
		Group:       in.Group,
		SessionID:   sessionID,
		Path:        in.Path,
		Status:      in.Status,
		CreatedAt:   at.In(time.Local).Format(time.RFC3339),
		Node:        in.Node,
		RequestJSON: reqObj,
		UserTurn:    UserTurn(messages),
		Truncated: TruncationFlags{
			Request:  in.RequestTrunc,
			Response: in.ResponseTrunc,
		},
	}

	ct := strings.ToLower(in.ResponseCT)
	isStream := strings.Contains(ct, "text/event-stream") || strings.Contains(ct, "text/eventstream")
	rec.IsStream = isStream
	if isStream {
		rec.ResponseSSE = string(in.ResponseBody)
	} else if len(in.ResponseBody) > 0 {
		var respObj any
		if err := common.Unmarshal(in.ResponseBody, &respObj); err != nil {
			rec.ResponseContent = map[string]any{
				"_raw":         string(in.ResponseBody),
				"_parse_error": err.Error(),
			}
		} else {
			rec.ResponseContent = SanitizeJSONValue(respObj)
		}
	}

	if sessionID != NoSessionID {
		if prevID, prevMsgs, ok := LoadPreviousMessages(in.UserID, sessionID, path); ok {
			if delta, matched := MessagesDelta(prevMsgs, messages); matched {
				rec.MessagesDelta = delta
				rec.PrevRequestID = prevID
			}
		}
	}
	if rec.MessagesDelta == nil {
		rec.MessagesDelta = []any{}
	}

	return rec, path, messages, nil
}

// ArchiveAsync builds and appends a record as one JSONL line.
func ArchiveAsync(in BuildInput) error {
	rec, path, messages, err := BuildRecord(in)
	if err != nil {
		return err
	}
	payload, err := common.Marshal(rec)
	if err != nil {
		return err
	}
	if err := AppendRecordJSONL(path, payload); err != nil {
		return err
	}
	RememberLastRecord(in.UserID, rec.SessionID, rec.RequestID, messages)
	return nil
}
