package model

import "github.com/gin-gonic/gin"

const requestConsumeLogKey = "request_consume_log"

// RequestConsumeLog is the settlement snapshot used to build the merged request log.
type RequestConsumeLog struct {
	UserID int
	Params RecordConsumeLogParams
	Other  map[string]any
}

func RememberRequestConsumeLog(c *gin.Context, userId int, params RecordConsumeLogParams) {
	if c == nil {
		return
	}
	snapshot := RequestConsumeLog{
		UserID: userId,
		Params: params,
	}
	if params.Other != nil {
		snapshot.Other = params.Other.Snapshot()
	}
	c.Set(requestConsumeLogKey, snapshot)
}

// RequestConsumeLogFrom returns the settlement snapshot stored for this request.
func RequestConsumeLogFrom(keys map[string]any) (RequestConsumeLog, bool) {
	if keys == nil {
		return RequestConsumeLog{}, false
	}
	snapshot, ok := keys[requestConsumeLogKey].(RequestConsumeLog)
	return snapshot, ok
}
