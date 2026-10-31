package model

import (
	"errors"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateUserTokenDistinguishesUnavailableStates(t *testing.T) {
	truncateTables(t)
	user := &User{
		Username: "token-validate-user",
		Password: "password-placeholder",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		AffCode:  "tv-aff",
	}
	require.NoError(t, DB.Create(user).Error)

	cases := []struct {
		name    string
		token   Token
		wantErr error
	}{
		{
			name: "disabled",
			token: Token{
				UserId: user.Id, Key: "tv-disabled", Name: "disabled",
				Status: common.TokenStatusDisabled, ExpiredTime: -1, UnlimitedQuota: true,
			},
			wantErr: ErrTokenDisabled,
		},
		{
			name: "expired status",
			token: Token{
				UserId: user.Id, Key: "tv-expired-status", Name: "expired-status",
				Status: common.TokenStatusExpired, ExpiredTime: -1, UnlimitedQuota: true,
			},
			wantErr: ErrTokenExpired,
		},
		{
			name: "exhausted status",
			token: Token{
				UserId: user.Id, Key: "tv-exhausted-status", Name: "exhausted-status",
				Status: common.TokenStatusExhausted, ExpiredTime: -1, UnlimitedQuota: true,
			},
			wantErr: ErrTokenExhausted,
		},
		{
			name: "time expired",
			token: Token{
				UserId: user.Id, Key: "tv-time-expired", Name: "time-expired",
				Status: common.TokenStatusEnabled, ExpiredTime: time.Now().Add(-time.Hour).Unix(), UnlimitedQuota: true,
			},
			wantErr: ErrTokenExpired,
		},
		{
			name: "quota exhausted",
			token: Token{
				UserId: user.Id, Key: "tv-quota-exhausted", Name: "quota-exhausted",
				Status: common.TokenStatusEnabled, ExpiredTime: -1, RemainQuota: 0, UnlimitedQuota: false,
			},
			wantErr: ErrTokenExhausted,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			token := tc.token
			require.NoError(t, DB.Create(&token).Error)
			got, err := ValidateUserToken(token.Key)
			require.Error(t, err)
			assert.True(t, errors.Is(err, tc.wantErr), "got %v", err)
			require.NotNil(t, got)
			assert.Equal(t, token.Id, got.Id)
			assert.Equal(t, user.Id, got.UserId)
		})
	}

	t.Run("not found", func(t *testing.T) {
		got, err := ValidateUserToken("tv-missing")
		assert.Nil(t, got)
		assert.True(t, errors.Is(err, ErrTokenInvalid), "got %v", err)
	})
}
