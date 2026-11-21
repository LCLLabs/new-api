package common

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func useVerificationRedis(t *testing.T) *miniredis.Miniredis {
	t.Helper()
	server := miniredis.RunT(t)
	previousEnabled := RedisEnabled
	previousClient := RDB
	RedisEnabled = true
	RDB = redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() {
		_ = RDB.Close()
		RedisEnabled = previousEnabled
		RDB = previousClient
	})
	return server
}

func useMemoryVerification(t *testing.T) {
	t.Helper()
	previousEnabled := RedisEnabled
	RedisEnabled = false
	verificationMutex.Lock()
	verificationMap = make(map[string]verificationValue)
	verificationMutex.Unlock()
	t.Cleanup(func() {
		RedisEnabled = previousEnabled
		verificationMutex.Lock()
		verificationMap = make(map[string]verificationValue)
		verificationMutex.Unlock()
	})
}

func TestMemoryVerificationCodeAcceptsTheIssuedCode(t *testing.T) {
	useMemoryVerification(t)

	require.NoError(t, RegisterVerificationCodeWithKey("user@example.com", "123456", EmailVerificationPurpose))

	assert.True(t, VerifyCodeWithKey("user@example.com", "123456", EmailVerificationPurpose))
	assert.False(t, VerifyCodeWithKey("user@example.com", "000000", EmailVerificationPurpose))
	assert.False(t, VerifyCodeWithKey("user@example.com", "123456", PasswordResetPurpose))
}

func TestMemoryVerificationCodeExpiresAndCanBeDeleted(t *testing.T) {
	useMemoryVerification(t)
	require.NoError(t, RegisterVerificationCodeWithKey("user@example.com", "123456", EmailVerificationPurpose))

	verificationMutex.Lock()
	entry := verificationMap[EmailVerificationPurpose+"user@example.com"]
	entry.time = time.Now().Add(-time.Duration(VerificationValidMinutes)*time.Minute - time.Second)
	verificationMap[EmailVerificationPurpose+"user@example.com"] = entry
	verificationMutex.Unlock()
	assert.False(t, VerifyCodeWithKey("user@example.com", "123456", EmailVerificationPurpose))

	require.NoError(t, RegisterVerificationCodeWithKey("user@example.com", "654321", EmailVerificationPurpose))
	DeleteKey("user@example.com", EmailVerificationPurpose)
	assert.False(t, VerifyCodeWithKey("user@example.com", "654321", EmailVerificationPurpose))
}

func TestRedisVerificationCodeIsVisibleWithoutLocalMemory(t *testing.T) {
	server := useVerificationRedis(t)
	verificationMutex.Lock()
	verificationMap = make(map[string]verificationValue)
	verificationMutex.Unlock()

	require.NoError(t, RegisterVerificationCodeWithKey("user@example.com", "123456", EmailVerificationPurpose))
	verificationMutex.Lock()
	require.Empty(t, verificationMap)
	verificationMutex.Unlock()

	stored, err := server.Get(verificationRedisKey(EmailVerificationPurpose, "user@example.com"))
	require.NoError(t, err)
	assert.NotEqual(t, "123456", stored)
	assert.Equal(t, verificationCodeMAC(EmailVerificationPurpose, "123456"), stored)

	assert.True(t, VerifyCodeWithKey("user@example.com", "123456", EmailVerificationPurpose))
	assert.False(t, VerifyCodeWithKey("user@example.com", "000000", EmailVerificationPurpose))
	assert.False(t, VerifyCodeWithKey("other@example.com", "123456", EmailVerificationPurpose))
	assert.False(t, VerifyCodeWithKey("user@example.com", "123456", PasswordResetPurpose))

	require.NoError(t, RegisterVerificationCodeWithKey("user@example.com", "654321", EmailVerificationPurpose))
	assert.False(t, VerifyCodeWithKey("user@example.com", "123456", EmailVerificationPurpose))
	assert.True(t, VerifyCodeWithKey("user@example.com", "654321", EmailVerificationPurpose))

	DeleteKey("user@example.com", EmailVerificationPurpose)
	assert.False(t, VerifyCodeWithKey("user@example.com", "654321", EmailVerificationPurpose))
}

func TestRedisVerificationCodeExpires(t *testing.T) {
	server := useVerificationRedis(t)
	previousMinutes := VerificationValidMinutes
	VerificationValidMinutes = 10
	t.Cleanup(func() {
		VerificationValidMinutes = previousMinutes
	})

	require.NoError(t, RegisterVerificationCodeWithKey("user@example.com", "123456", PasswordResetPurpose))
	server.FastForward(10*time.Minute + time.Second)
	assert.False(t, VerifyCodeWithKey("user@example.com", "123456", PasswordResetPurpose))
}

func TestRedisVerificationStoreFailureDoesNotAcceptTheCode(t *testing.T) {
	server := useVerificationRedis(t)
	server.Close()

	err := RegisterVerificationCodeWithKey("user@example.com", "123456", EmailVerificationPurpose)
	require.Error(t, err)
	assert.False(t, VerifyCodeWithKey("user@example.com", "123456", EmailVerificationPurpose))
}
