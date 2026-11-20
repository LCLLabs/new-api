package common

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
)

type verificationValue struct {
	code string
	time time.Time
}

const (
	EmailVerificationPurpose = "v"
	PasswordResetPurpose     = "r"
)

var verificationMutex sync.Mutex
var verificationMap map[string]verificationValue
var verificationMapMaxSize = 10
var VerificationValidMinutes = 10

func GenerateVerificationCode(length int) string {
	code := uuid.New().String()
	code = strings.ReplaceAll(code, "-", "")
	if length == 0 {
		return code
	}
	return code[:length]
}

// RegisterVerificationCodeWithKey stores a code for key. Redis keeps a MAC of
// the code so every node in a cluster can verify the mail that any node sent.
// The raw code is not written to Redis or to logs.
func RegisterVerificationCodeWithKey(key string, code string, purpose string) error {
	if RedisEnabled {
		return registerRedisVerificationCode(key, code, purpose)
	}
	registerMemoryVerificationCode(key, code, purpose)
	return nil
}

func VerifyCodeWithKey(key string, code string, purpose string) bool {
	if RedisEnabled {
		return verifyRedisVerificationCode(key, code, purpose)
	}
	return verifyMemoryVerificationCode(key, code, purpose)
}

func DeleteKey(key string, purpose string) {
	if RedisEnabled {
		deleteRedisVerificationCode(key, purpose)
		return
	}
	deleteMemoryVerificationCode(key, purpose)
}

func registerMemoryVerificationCode(key string, code string, purpose string) {
	verificationMutex.Lock()
	defer verificationMutex.Unlock()
	verificationMap[purpose+key] = verificationValue{
		code: code,
		time: time.Now(),
	}
	if len(verificationMap) > verificationMapMaxSize {
		removeExpiredPairs()
	}
}

func verifyMemoryVerificationCode(key string, code string, purpose string) bool {
	verificationMutex.Lock()
	defer verificationMutex.Unlock()
	value, okay := verificationMap[purpose+key]
	now := time.Now()
	if !okay || int(now.Sub(value.time).Seconds()) >= VerificationValidMinutes*60 {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(code), []byte(value.code)) == 1
}

func deleteMemoryVerificationCode(key string, purpose string) {
	verificationMutex.Lock()
	defer verificationMutex.Unlock()
	delete(verificationMap, purpose+key)
}

func verificationRedisKey(purpose string, key string) string {
	sum := sha256.Sum256([]byte(purpose + "\x00" + key))
	return "verification:" + hex.EncodeToString(sum[:])
}

func verificationCodeMAC(purpose string, code string) string {
	return GenerateHMAC(purpose + ":" + code)
}

func verificationCodeTTL() (time.Duration, error) {
	if VerificationValidMinutes <= 0 {
		return 0, errors.New("verification code lifetime is not positive")
	}
	return time.Duration(VerificationValidMinutes) * time.Minute, nil
}

func registerRedisVerificationCode(key string, code string, purpose string) error {
	if RDB == nil {
		return errors.New("redis client is not initialized")
	}
	ttl, err := verificationCodeTTL()
	if err != nil {
		return err
	}
	// Do not use RedisSet: its debug log prints the stored value.
	return RDB.Set(context.Background(), verificationRedisKey(purpose, key), verificationCodeMAC(purpose, code), ttl).Err()
}

func verifyRedisVerificationCode(key string, code string, purpose string) bool {
	if RDB == nil || VerificationValidMinutes <= 0 {
		return false
	}
	stored, err := RDB.Get(context.Background(), verificationRedisKey(purpose, key)).Result()
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			SysLog(LogText("failed to read verification code: %s", err.Error()))
		}
		return false
	}
	expected := verificationCodeMAC(purpose, code)
	return subtle.ConstantTimeCompare([]byte(stored), []byte(expected)) == 1
}

func deleteRedisVerificationCode(key string, purpose string) {
	if RDB == nil {
		return
	}
	if err := RDB.Del(context.Background(), verificationRedisKey(purpose, key)).Err(); err != nil {
		SysLog(LogText("failed to delete verification code: %s", err.Error()))
	}
}

// no lock inside, so the caller must lock the verificationMap before calling!
func removeExpiredPairs() {
	now := time.Now()
	for key := range verificationMap {
		if int(now.Sub(verificationMap[key].time).Seconds()) >= VerificationValidMinutes*60 {
			delete(verificationMap, key)
		}
	}
}

func init() {
	verificationMutex.Lock()
	defer verificationMutex.Unlock()
	verificationMap = make(map[string]verificationValue)
}
