package middleware

import (
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/service/promptarchive"
	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
)

type promptArchiveTeeWriter struct {
	gin.ResponseWriter
	buf       []byte
	max       int
	truncated bool
}

func (w *promptArchiveTeeWriter) Write(data []byte) (int, error) {
	w.capture(data)
	return w.ResponseWriter.Write(data)
}

func (w *promptArchiveTeeWriter) WriteString(s string) (int, error) {
	w.capture([]byte(s))
	if ws, ok := w.ResponseWriter.(interface{ WriteString(string) (int, error) }); ok {
		return ws.WriteString(s)
	}
	return w.ResponseWriter.Write([]byte(s))
}

func (w *promptArchiveTeeWriter) capture(data []byte) {
	if w.truncated || len(data) == 0 {
		return
	}
	remain := w.max - len(w.buf)
	if remain <= 0 {
		w.truncated = true
		return
	}
	if len(data) > remain {
		w.buf = append(w.buf, data[:remain]...)
		w.truncated = true
		return
	}
	w.buf = append(w.buf, data...)
}

// PromptArchive captures eligible text relay request/response bodies to a local directory.
func PromptArchive() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !common.PromptArchiveEnabled {
			c.Next()
			return
		}
		path := c.Request.URL.Path
		if !promptarchive.PathEligible(path) {
			c.Next()
			return
		}

		tee := &promptArchiveTeeWriter{
			ResponseWriter: c.Writer,
			buf:            make([]byte, 0, 4096),
			max:            common.PromptArchiveMaxResponseBytes,
		}
		c.Writer = tee
		c.Next()

		requestBody, reqTrunc := copyRequestBody(c)
		respCopy, respTrunc := promptarchive.CopyBytesCap(tee.buf, common.PromptArchiveMaxResponseBytes)
		if tee.truncated {
			respTrunc = true
		}

		in := promptarchive.BuildInput{
			RequestID:     c.GetString(common.RequestIdKey),
			UserID:        c.GetInt("id"),
			TokenID:       common.GetContextKeyInt(c, constant.ContextKeyTokenId),
			ChannelID:     common.GetContextKeyInt(c, constant.ContextKeyChannelId),
			ModelName:     common.GetContextKeyString(c, constant.ContextKeyOriginalModel),
			Group:         common.GetContextKeyString(c, constant.ContextKeyUsingGroup),
			Path:          path,
			Status:        c.Writer.Status(),
			CreatedAt:     time.Now(),
			Node:          common.NodeName,
			HeaderSession: c.GetHeader("X-Session-Id"),
			HeaderConv:    c.GetHeader("X-Conversation-Id"),
			RequestBody:   requestBody,
			RequestTrunc:  reqTrunc,
			ResponseBody:  respCopy,
			ResponseTrunc: respTrunc,
			ResponseCT:    c.Writer.Header().Get("Content-Type"),
		}
		gopool.Go(func() {
			if err := promptarchive.ArchiveAsync(in); err != nil {
				logger.LogWarn(nil, "prompt archive write failed request_id=%s: %s", in.RequestID, err.Error())
			}
		})
	}
}

func copyRequestBody(c *gin.Context) ([]byte, bool) {
	storage, err := common.GetBodyStorage(c)
	if err != nil || storage == nil {
		return nil, false
	}
	raw, err := storage.Bytes()
	if err != nil {
		return nil, false
	}
	return promptarchive.CopyBytesCap(raw, common.PromptArchiveMaxRequestBytes)
}
