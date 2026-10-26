package promptarchive

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
)

type lastRecord struct {
	RequestID string
	Messages  []any
}

var (
	lastRecordCache sync.Map // key: "userID/sessionID" -> lastRecord
	fileAppendMu    sync.Map // key: path -> *sync.Mutex
)

func sessionCacheKey(userID int, sessionID string) string {
	return fmt.Sprintf("%d/%s", userID, sessionID)
}

func fileMutex(path string) *sync.Mutex {
	v, _ := fileAppendMu.LoadOrStore(path, &sync.Mutex{})
	return v.(*sync.Mutex)
}

// ArchivePath builds by-user/{user}/{session}/{YYYY-MM-DD}.jsonl under root.
// One file per user+session+day; each request is one JSON line.
func ArchivePath(root string, userID int, sessionID string, at time.Time) string {
	if root == "" {
		root = common.PromptArchiveDir
	}
	if sessionID == "" {
		sessionID = NoSessionID
	}
	day := at.In(time.Local).Format("2006-01-02")
	return filepath.Join(root, "by-user", fmt.Sprintf("%d", userID), sessionID, day+".jsonl")
}

// RememberLastRecord stores the latest messages for session delta on this process.
func RememberLastRecord(userID int, sessionID, requestID string, messages []any) {
	if sessionID == "" || sessionID == NoSessionID || requestID == "" {
		return
	}
	lastRecordCache.Store(sessionCacheKey(userID, sessionID), lastRecord{
		RequestID: requestID,
		Messages:  messages,
	})
}

// LoadPreviousMessages returns the previous turn in the same session for delta.
func LoadPreviousMessages(userID int, sessionID string, dayFile string) (requestID string, messages []any, ok bool) {
	if sessionID == "" || sessionID == NoSessionID {
		return "", nil, false
	}
	if v, hit := lastRecordCache.Load(sessionCacheKey(userID, sessionID)); hit {
		if prev, castOK := v.(lastRecord); castOK && len(prev.Messages) > 0 {
			return prev.RequestID, prev.Messages, true
		}
	}
	return loadLastLineRecord(dayFile)
}

func loadLastLineRecord(path string) (requestID string, messages []any, ok bool) {
	if path == "" {
		return "", nil, false
	}
	line, err := readLastNonEmptyLine(path)
	if err != nil || len(line) == 0 {
		return "", nil, false
	}
	var rec map[string]any
	if err := common.Unmarshal(line, &rec); err != nil {
		return "", nil, false
	}
	if id, _ := rec["request_id"].(string); id != "" {
		requestID = id
	}
	messages = ExtractMessages(rec["request_json"])
	if messages == nil {
		return requestID, nil, false
	}
	return requestID, messages, true
}

func readLastNonEmptyLine(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	size := info.Size()
	if size == 0 {
		return nil, nil
	}

	const maxScan = 8 << 20 // 8MiB tail window
	start := int64(0)
	if size > maxScan {
		start = size - maxScan
	}
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), common.PromptArchiveMaxRequestBytes+common.PromptArchiveMaxResponseBytes+64*1024)
	var last []byte
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		last = append(last[:0], line...)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if start > 0 && len(last) == 0 {
		// Tail window started mid-line and found nothing complete; fall back to full scan.
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return nil, err
		}
		scanner = bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 0, 64*1024), common.PromptArchiveMaxRequestBytes+common.PromptArchiveMaxResponseBytes+64*1024)
		for scanner.Scan() {
			line := bytes.TrimSpace(scanner.Bytes())
			if len(line) == 0 {
				continue
			}
			last = append(last[:0], line...)
		}
		if err := scanner.Err(); err != nil {
			return nil, err
		}
	}
	return last, nil
}

// AppendRecordJSONL appends one JSON object as a line to path.
func AppendRecordJSONL(path string, payload []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	mu := fileMutex(path)
	mu.Lock()
	defer mu.Unlock()

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.Write(payload); err != nil {
		return err
	}
	if _, err := f.Write([]byte("\n")); err != nil {
		return err
	}
	return nil
}

// ResetLastPathCacheForTest clears in-process caches used by archive writers.
func ResetLastPathCacheForTest() {
	lastRecordCache = sync.Map{}
	fileAppendMu = sync.Map{}
}

// CountJSONLLines counts non-empty lines in a jsonl file (tests / ops helper).
func CountJSONLLines(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	n := 0
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) != "" {
			n++
		}
	}
	return n, scanner.Err()
}
