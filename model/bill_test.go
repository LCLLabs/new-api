package model

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestBuildUserBillGroupsRatioUsageAndRefunds(t *testing.T) {
	vendors := map[string]billVendorRef{
		"claude-sonnet": {ID: 7, Name: "Anthropic", Icon: "Claude"},
	}
	bill := BuildUserBill([]BillSource{
		{
			Type:             LogTypeConsume,
			ModelName:        "claude-sonnet",
			TokenName:        "trial",
			TokenID:          3,
			Group:            "default",
			Quota:            500000,
			PromptTokens:     1500,
			CompletionTokens: 20,
			Other:            `{"group_ratio":1,"user_group_ratio":0.5,"cache_tokens":200,"cache_creation_tokens_5m":300,"cache_write_tokens":300,"input_tokens_total":1500}`,
		},
		{
			Type:      LogTypeConsume,
			ModelName: "claude-sonnet",
			TokenName: "trial",
			TokenID:   3,
			Group:     "default",
			Quota:     500000,
			Other:     `{"group_ratio":1,"user_group_ratio":0.5}`,
		},
		{
			Type:      LogTypeRefund,
			Quota:     100000,
			ModelName: "",
		},
		{
			Type:  LogTypeTopup,
			Quota: 999,
		},
	}, vendors, BillFilter{})

	assert.Nil(t, bill.Total.Discount)
	assert.Nil(t, bill.Total.ListQuota)
	assert.Equal(t, int64(900000), bill.Total.ActualQuota)
	assert.Equal(t, 2, bill.Total.RequestCount)
	assert.Equal(t, int64(1000), bill.Total.UncachedInput)
	assert.Equal(t, int64(200), bill.Total.CacheRead)
	assert.Equal(t, int64(300), bill.Total.CacheWrite5m)
	assert.Equal(t, int64(0), bill.Total.CacheWriteOther)
	assert.Equal(t, int64(20), bill.Total.OutputTokens)

	require.Len(t, bill.Rows, 2)
	assert.Equal(t, "claude-sonnet", bill.Rows[0].ModelName)
	assert.Equal(t, "Anthropic", bill.Rows[0].VendorName)
	assert.True(t, bill.Rows[1].Adjustment)
	assert.Equal(t, int64(-100000), bill.Rows[1].ActualQuota)

	require.Len(t, bill.Vendors, 1)
	assert.Equal(t, int64(1000000), bill.Vendors[0].ActualQuota)
	require.NotNil(t, bill.Vendors[0].Discount)
	assert.InDelta(t, 0.5, *bill.Vendors[0].Discount, 0.000001)
	require.NotNil(t, bill.Vendors[0].ListQuota)
	assert.Equal(t, int64(2000000), *bill.Vendors[0].ListQuota)
	assert.Equal(t, []string{"claude-sonnet"}, bill.Facets.Models)
	assert.Equal(t, []string{"trial"}, bill.Facets.Tokens)
	assert.Equal(t, []string{"default"}, bill.Facets.Groups)
}

func TestBuildUserBillSumsListPriceWhenDiscountsDiffer(t *testing.T) {
	bill := BuildUserBill([]BillSource{
		{Type: LogTypeConsume, ModelName: "m", TokenName: "a", Group: "g", Quota: 100, Other: `{"group_ratio":1}`},
		{Type: LogTypeConsume, ModelName: "m", TokenName: "a", Group: "g", Quota: 100, Other: `{"group_ratio":0.5}`},
	}, nil, BillFilter{})

	require.Len(t, bill.Rows, 2)
	assert.InDelta(t, 0.5, *bill.Rows[0].Discount, 0.000001)
	assert.Equal(t, int64(200), *bill.Rows[0].ListQuota)
	assert.InDelta(t, 1, *bill.Rows[1].Discount, 0.000001)
	assert.Equal(t, int64(100), *bill.Rows[1].ListQuota)
	require.NotNil(t, bill.Total.ListQuota)
	assert.Equal(t, int64(300), *bill.Total.ListQuota)
	assert.Nil(t, bill.Total.Discount)
	assert.False(t, bill.Total.Dynamic)
	assert.Equal(t, int64(200), bill.Total.ActualQuota)
}

func TestBuildUserBillPricesTieredExprFromGroupRatioOnly(t *testing.T) {
	bill := BuildUserBill([]BillSource{
		{
			Type:      LogTypeConsume,
			ModelName: "m",
			Quota:     80,
			Other:     `{"group_ratio":0.8,"user_group_ratio":-1,"billing_mode":"tiered_expr"}`,
		},
		{
			Type:      LogTypeRefund,
			ModelName: "m",
			Quota:     40,
			Other:     `{"group_ratio":0.8,"user_group_ratio":0.8,"billing_mode":"tiered_expr"}`,
		},
	}, nil, BillFilter{})

	assert.Equal(t, int64(40), bill.Total.ActualQuota)
	require.NotNil(t, bill.Total.ListQuota)
	assert.Equal(t, int64(50), *bill.Total.ListQuota)
	require.NotNil(t, bill.Total.Discount)
	assert.InDelta(t, 0.8, *bill.Total.Discount, 0.000001)
	assert.True(t, bill.Total.Dynamic)
}

func TestBuildUserBillIgnoresNegativeSentinelOnRatioBilling(t *testing.T) {
	bill := BuildUserBill([]BillSource{{
		Type: LogTypeConsume, ModelName: "m", Quota: 80,
		Other: `{"group_ratio":0.8,"user_group_ratio":-1,"model_price":0.02}`,
	}}, nil, BillFilter{})

	assert.Equal(t, int64(80), bill.Total.ActualQuota)
	require.NotNil(t, bill.Total.ListQuota)
	assert.Equal(t, int64(100), *bill.Total.ListQuota)
	require.NotNil(t, bill.Total.Discount)
	assert.InDelta(t, 0.8, *bill.Total.Discount, 0.000001)
	assert.False(t, bill.Total.Dynamic)
}

func TestBuildUserBillLeavesPriceBlankForInvalidRatio(t *testing.T) {
	cases := []string{
		`{"group_ratio":0,"billing_mode":"tiered_expr"}`,
		`{"group_ratio":-1}`,
		`{"group_ratio":1,"user_group_ratio":0}`,
		`{"group_ratio":1,"user_group_ratio":-2}`,
		`{}`,
	}
	for _, other := range cases {
		bill := BuildUserBill([]BillSource{{
			Type: LogTypeConsume, ModelName: "m", Quota: 80, Other: other,
		}}, nil, BillFilter{})
		assert.Nil(t, bill.Total.Discount, other)
		assert.Nil(t, bill.Total.ListQuota, other)
		assert.Equal(t, int64(80), bill.Total.ActualQuota, other)
	}
}

func TestBuildUserBillSplitsUnlabeledCacheWrite(t *testing.T) {
	bill := BuildUserBill([]BillSource{{
		Type:         LogTypeConsume,
		ModelName:    "m",
		PromptTokens: 1000,
		Other:        `{"group_ratio":1,"cache_tokens":100,"cache_creation_tokens_5m":50,"cache_creation_tokens_1h":20,"cache_creation_tokens_12h":10,"cache_write_tokens":90}`,
	}}, nil, BillFilter{})

	assert.Equal(t, int64(50), bill.Total.CacheWrite5m)
	assert.Equal(t, int64(20), bill.Total.CacheWrite1h)
	assert.Equal(t, int64(20), bill.Total.CacheWriteOther)
	assert.Equal(t, int64(810), bill.Total.UncachedInput)
}

func TestBuildUserBillFiltersVendorWithoutHidingFacets(t *testing.T) {
	vendors := map[string]billVendorRef{
		"a": {ID: 1, Name: "Anthropic"},
		"b": {ID: 2, Name: "OpenAI"},
	}
	vendorID := 2
	bill := BuildUserBill([]BillSource{
		{Type: LogTypeConsume, ModelName: "a", TokenName: "one", Group: "g", Quota: 10, Other: `{"group_ratio":1}`},
		{Type: LogTypeConsume, ModelName: "b", TokenName: "two", Group: "h", Quota: 30, Other: `{"group_ratio":1}`},
	}, vendors, BillFilter{VendorID: &vendorID})

	require.Len(t, bill.Rows, 1)
	assert.Equal(t, "b", bill.Rows[0].ModelName)
	assert.Equal(t, int64(30), bill.Total.ActualQuota)
	assert.Equal(t, []string{"a", "b"}, bill.Facets.Models)
	require.Len(t, bill.Facets.Vendors, 2)
}

func TestQueryUserBillReadsStoredLogs(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	previousDB, previousLogDB := DB, LOG_DB
	DB, LOG_DB = db, db
	initCol()
	t.Cleanup(func() {
		DB, LOG_DB = previousDB, previousLogDB
		initCol()
	})
	require.NoError(t, db.AutoMigrate(&Log{}, &Model{}, &Vendor{}))

	vendor := &Vendor{Name: "Anthropic", Icon: "Claude", Status: 1}
	require.NoError(t, db.Create(vendor).Error)
	require.NoError(t, db.Create(&Model{ModelName: "claude-sonnet", VendorID: vendor.Id}).Error)
	require.NoError(t, db.Create(&Log{
		UserId:           4,
		CreatedAt:        1_700_000_000,
		Type:             LogTypeConsume,
		ModelName:        "claude-sonnet",
		TokenName:        "trial",
		TokenId:          9,
		Group:            "default",
		Quota:            250000,
		PromptTokens:     40,
		CompletionTokens: 5,
		Other:            `{"group_ratio":1,"user_group_ratio":1,"cache_tokens":10}`,
	}).Error)
	require.NoError(t, db.Create(&Log{
		UserId:    4,
		CreatedAt: 1_700_000_100,
		Type:      LogTypeRefund,
		Quota:     50000,
	}).Error)
	require.NoError(t, db.Create(&Log{
		UserId:    8,
		CreatedAt: 1_700_000_000,
		Type:      LogTypeConsume,
		ModelName: "claude-sonnet",
		Quota:     999,
		Other:     `{"group_ratio":1}`,
	}).Error)

	bill, err := QueryUserBill(context.Background(), 4, 1_700_000_000, 1_700_000_100, BillFilter{})
	require.NoError(t, err)
	assert.Equal(t, int64(200000), bill.Total.ActualQuota)
	assert.Equal(t, 1, bill.Total.RequestCount)
	assert.Equal(t, int64(30), bill.Total.UncachedInput)
	require.Len(t, bill.Rows, 2)
	assert.Equal(t, "Anthropic", bill.Rows[0].VendorName)
	assert.Equal(t, "Claude", bill.Rows[0].VendorIcon)
	assert.True(t, bill.Rows[1].Adjustment)
}
