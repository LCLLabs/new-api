package model

import (
	"context"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// BillFilter selects one user's statement after the logs have been grouped.
// A nil VendorID keeps every vendor, including rows whose model has none.
type BillFilter struct {
	VendorID  *int
	ModelName string
	TokenName string
	Group     string
}

// BillMetrics is one statement total. ListQuota and Discount are set only when
// every included request shares one group discount and is ratio billing.
type BillMetrics struct {
	RequestCount    int      `json:"request_count"`
	UncachedInput   int64    `json:"uncached_input"`
	CacheRead       int64    `json:"cache_read"`
	CacheWrite5m    int64    `json:"cache_write_5m"`
	CacheWrite1h    int64    `json:"cache_write_1h"`
	CacheWriteOther int64    `json:"cache_write_other"`
	OutputTokens    int64    `json:"output_tokens"`
	ActualQuota     int64    `json:"actual_quota"`
	ListQuota       *int64   `json:"list_quota,omitempty"`
	Discount        *float64 `json:"discount,omitempty"`
}

// BillVendorTotal is the statement subtotal for one model vendor.
type BillVendorTotal struct {
	VendorID   int    `json:"vendor_id"`
	VendorName string `json:"vendor_name"`
	VendorIcon string `json:"vendor_icon,omitempty"`
	BillMetrics
}

// BillDetail is one statement row: vendor, model, token, and group.
type BillDetail struct {
	Adjustment bool   `json:"adjustment"`
	VendorID   int    `json:"vendor_id"`
	VendorName string `json:"vendor_name"`
	VendorIcon string `json:"vendor_icon,omitempty"`
	ModelName  string `json:"model_name"`
	TokenName  string `json:"token_name"`
	TokenID    int    `json:"token_id"`
	Group      string `json:"group"`
	BillMetrics
}

// BillFacetVendor is a vendor the user can filter by.
type BillFacetVendor struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Icon string `json:"icon,omitempty"`
}

// BillFacets lists the values present in the time range before the other filters.
type BillFacets struct {
	Vendors []BillFacetVendor `json:"vendors"`
	Models  []string          `json:"models"`
	Tokens  []string          `json:"tokens"`
	Groups  []string          `json:"groups"`
}

// UserBill is a personal statement calculated from consume and refund logs.
type UserBill struct {
	Total   BillMetrics       `json:"total"`
	Vendors []BillVendorTotal `json:"vendors"`
	Rows    []BillDetail      `json:"rows"`
	Facets  BillFacets        `json:"facets"`
}

// BillSource is the log columns the statement reads. Other is the stored JSON text.
type BillSource struct {
	Type             int
	ModelName        string
	TokenName        string
	TokenID          int
	Group            string
	Quota            int
	PromptTokens     int
	CompletionTokens int
	Other            string
}

type billVendorRef struct {
	ID   int
	Name string
	Icon string
}

type billBucket struct {
	adjustment bool
	vendorID   int
	vendorName string
	vendorIcon string
	modelName  string
	tokenName  string
	tokenID    int
	group      string
	usage      billUsage
}

type billUsage struct {
	requests        int
	uncachedInput   int64
	cacheRead       int64
	cacheWrite5m    int64
	cacheWrite1h    int64
	cacheWriteOther int64
	outputTokens    int64
	actualQuota     int64
	ratio           ratioTrack
}

type ratioTrack struct {
	set   bool
	mixed bool
	value float64
}

type parsedBillOther struct {
	uncachedInput   int64
	cacheRead       int64
	cacheWrite5m    int64
	cacheWrite1h    int64
	cacheWriteOther int64
	ratio           float64
	ratioOK         bool
}

func (track *ratioTrack) observe(value float64, ok bool) {
	if track.mixed {
		return
	}
	if !ok || value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		track.mixed = true
		track.set = false
		return
	}
	if !track.set {
		track.set = true
		track.value = value
		return
	}
	if math.Abs(track.value-value) > 1e-6 {
		track.mixed = true
		track.set = false
	}
}

func (track *ratioTrack) merge(other ratioTrack) {
	if other.mixed || !other.set {
		track.observe(0, false)
		return
	}
	track.observe(other.value, true)
}

func (usage *billUsage) add(source BillSource, parsed parsedBillOther, sign int64) {
	if source.Type == LogTypeConsume {
		usage.requests++
	}
	usage.uncachedInput = addInt64(usage.uncachedInput, sign*parsed.uncachedInput)
	usage.cacheRead = addInt64(usage.cacheRead, sign*parsed.cacheRead)
	usage.cacheWrite5m = addInt64(usage.cacheWrite5m, sign*parsed.cacheWrite5m)
	usage.cacheWrite1h = addInt64(usage.cacheWrite1h, sign*parsed.cacheWrite1h)
	usage.cacheWriteOther = addInt64(usage.cacheWriteOther, sign*parsed.cacheWriteOther)
	usage.outputTokens = addInt64(usage.outputTokens, sign*int64(source.CompletionTokens))
	usage.actualQuota = addInt64(usage.actualQuota, sign*int64(source.Quota))
	usage.ratio.observe(parsed.ratio, parsed.ratioOK)
}

func (usage *billUsage) merge(other billUsage) {
	usage.requests += other.requests
	usage.uncachedInput = addInt64(usage.uncachedInput, other.uncachedInput)
	usage.cacheRead = addInt64(usage.cacheRead, other.cacheRead)
	usage.cacheWrite5m = addInt64(usage.cacheWrite5m, other.cacheWrite5m)
	usage.cacheWrite1h = addInt64(usage.cacheWrite1h, other.cacheWrite1h)
	usage.cacheWriteOther = addInt64(usage.cacheWriteOther, other.cacheWriteOther)
	usage.outputTokens = addInt64(usage.outputTokens, other.outputTokens)
	usage.actualQuota = addInt64(usage.actualQuota, other.actualQuota)
	usage.ratio.merge(other.ratio)
}

func (usage billUsage) metrics() BillMetrics {
	metrics := BillMetrics{
		RequestCount:    usage.requests,
		UncachedInput:   usage.uncachedInput,
		CacheRead:       usage.cacheRead,
		CacheWrite5m:    usage.cacheWrite5m,
		CacheWrite1h:    usage.cacheWrite1h,
		CacheWriteOther: usage.cacheWriteOther,
		OutputTokens:    usage.outputTokens,
		ActualQuota:     usage.actualQuota,
	}
	if !usage.ratio.set || usage.ratio.mixed {
		return metrics
	}
	list, ok := listQuotaFromActual(usage.actualQuota, usage.ratio.value)
	if !ok {
		return metrics
	}
	discount := math.Round(usage.ratio.value*1e6) / 1e6
	metrics.ListQuota = &list
	metrics.Discount = &discount
	return metrics
}

// listQuotaFromActual reconstructs the pre-discount amount for display.
// It is not a charge: settlement already stored ActualQuota.
func listQuotaFromActual(actual int64, ratio float64) (int64, bool) {
	if ratio <= 0 || math.IsNaN(ratio) || math.IsInf(ratio, 0) {
		return 0, false
	}
	if ratio == 1 {
		return actual, true
	}
	value := float64(actual) / ratio
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, false
	}
	if value >= float64(math.MaxInt64) || value <= float64(math.MinInt64) {
		return 0, false
	}
	return int64(math.Round(value)), true
}

func addInt64(sum int64, delta int64) int64 {
	if delta > 0 && sum > math.MaxInt64-delta {
		return math.MaxInt64
	}
	if delta < 0 && sum < math.MinInt64-delta {
		return math.MinInt64
	}
	return sum + delta
}

func parseBillOther(source BillSource) parsedBillOther {
	parsed := parsedBillOther{}
	fields := map[string]any{}
	if strings.TrimSpace(source.Other) != "" {
		if err := common.Unmarshal([]byte(source.Other), &fields); err != nil {
			fields = map[string]any{}
		}
	}
	cacheRead, _ := jsonNonNegative(fields["cache_tokens"])
	write5m, _ := jsonNonNegative(fields["cache_creation_tokens_5m"])
	write1h, _ := jsonNonNegative(fields["cache_creation_tokens_1h"])
	write12h, _ := jsonNonNegative(fields["cache_creation_tokens_12h"])
	writeTotal, hasWriteTotal := jsonNonNegative(fields["cache_write_tokens"])
	if !hasWriteTotal {
		writeTotal, hasWriteTotal = jsonNonNegative(fields["cache_creation_tokens"])
	}
	split := addInt64(addInt64(write5m, write1h), write12h)
	if !hasWriteTotal {
		writeTotal = split
	}
	otherWrite := write12h
	if writeTotal > split {
		otherWrite = addInt64(otherWrite, writeTotal-split)
	}

	input := int64(source.PromptTokens)
	if total, ok := jsonNonNegative(fields["input_tokens_total"]); ok {
		input = total
	}
	if input < 0 {
		input = 0
	}
	uncached := input - cacheRead - writeTotal
	if uncached < 0 {
		uncached = 0
	}
	parsed.uncachedInput = uncached
	parsed.cacheRead = cacheRead
	parsed.cacheWrite5m = write5m
	parsed.cacheWrite1h = write1h
	parsed.cacheWriteOther = otherWrite
	parsed.ratio, parsed.ratioOK = billDiscountRatio(fields)
	return parsed
}

func billDiscountRatio(fields map[string]any) (float64, bool) {
	if jsonBool(fields["is_task"]) || jsonBool(fields["audio_input_seperate_price"]) {
		return 0, false
	}
	if mode, _ := fields["billing_mode"].(string); mode == "tiered_expr" {
		return 0, false
	}
	if _, ok := jsonPositive(fields["model_price"]); ok {
		return 0, false
	}
	if surcharges, ok := fields["tool_surcharges"].([]any); ok && len(surcharges) > 0 {
		return 0, false
	}
	groupRatio, ok := jsonPositive(fields["group_ratio"])
	if !ok {
		return 0, false
	}
	userRatio := 1.0
	if _, present := fields["user_group_ratio"]; present {
		userRatio, ok = jsonPositive(fields["user_group_ratio"])
		if !ok {
			return 0, false
		}
	}
	ratio := groupRatio * userRatio
	if ratio <= 0 || math.IsNaN(ratio) || math.IsInf(ratio, 0) {
		return 0, false
	}
	return ratio, true
}

func jsonBool(value any) bool {
	flag, ok := value.(bool)
	return ok && flag
}

func jsonPositive(value any) (float64, bool) {
	number, ok := jsonFloat(value)
	if !ok || number <= 0 || math.IsNaN(number) || math.IsInf(number, 0) {
		return 0, false
	}
	return number, true
}

func jsonNonNegative(value any) (int64, bool) {
	if value == nil {
		return 0, false
	}
	number, ok := jsonFloat(value)
	if !ok || number < 0 || math.IsNaN(number) || math.IsInf(number, 0) || number >= float64(math.MaxInt64) {
		return 0, false
	}
	return int64(number), true
}

func jsonFloat(value any) (float64, bool) {
	switch number := value.(type) {
	case float64:
		return number, true
	case float32:
		return float64(number), true
	case int:
		return float64(number), true
	case int64:
		return float64(number), true
	case uint64:
		return float64(number), true
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(number), 64)
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}

func unscopedRefund(source BillSource) bool {
	return source.Type == LogTypeRefund &&
		source.ModelName == "" &&
		source.TokenName == "" &&
		source.TokenID == 0 &&
		source.Group == ""
}

type billAccumulator struct {
	buckets map[string]*billBucket
}

func newBillAccumulator() *billAccumulator {
	return &billAccumulator{buckets: map[string]*billBucket{}}
}

func (accumulator *billAccumulator) add(source BillSource, vendors map[string]billVendorRef) {
	if source.Type != LogTypeConsume && source.Type != LogTypeRefund {
		return
	}
	sign := int64(1)
	if source.Type == LogTypeRefund {
		sign = -1
	}
	adjustment := unscopedRefund(source)
	vendor := vendors[source.ModelName]
	if adjustment {
		vendor = billVendorRef{}
	}
	key := billBucketKey(adjustment, vendor.ID, source)
	bucket := accumulator.buckets[key]
	if bucket == nil {
		bucket = &billBucket{
			adjustment: adjustment,
			vendorID:   vendor.ID,
			vendorName: vendor.Name,
			vendorIcon: vendor.Icon,
			modelName:  source.ModelName,
			tokenName:  source.TokenName,
			tokenID:    source.TokenID,
			group:      source.Group,
		}
		accumulator.buckets[key] = bucket
	}
	bucket.usage.add(source, parseBillOther(source), sign)
}

func billBucketKey(adjustment bool, vendorID int, source BillSource) string {
	if adjustment {
		return "adjustment"
	}
	return strings.Join([]string{
		strconv.Itoa(vendorID),
		source.ModelName,
		strconv.Itoa(source.TokenID),
		source.TokenName,
		source.Group,
	}, "\x00")
}

func (accumulator *billAccumulator) finish(filter BillFilter) UserBill {
	all := make([]*billBucket, 0, len(accumulator.buckets))
	for _, bucket := range accumulator.buckets {
		all = append(all, bucket)
	}
	facets := billFacets(all)
	matched := make([]*billBucket, 0, len(all))
	for _, bucket := range all {
		if billBucketMatches(bucket, filter) {
			matched = append(matched, bucket)
		}
	}
	slices.SortFunc(matched, func(a, b *billBucket) int {
		if a.usage.actualQuota != b.usage.actualQuota {
			if a.usage.actualQuota > b.usage.actualQuota {
				return -1
			}
			return 1
		}
		return strings.Compare(a.modelName+"\x00"+a.tokenName+"\x00"+a.group, b.modelName+"\x00"+b.tokenName+"\x00"+b.group)
	})

	rows := make([]BillDetail, 0, len(matched))
	vendorUsage := map[int]*billBucket{}
	var total billUsage
	for _, bucket := range matched {
		rows = append(rows, BillDetail{
			Adjustment:  bucket.adjustment,
			VendorID:    bucket.vendorID,
			VendorName:  bucket.vendorName,
			VendorIcon:  bucket.vendorIcon,
			ModelName:   bucket.modelName,
			TokenName:   bucket.tokenName,
			TokenID:     bucket.tokenID,
			Group:       bucket.group,
			BillMetrics: bucket.usage.metrics(),
		})
		total.merge(bucket.usage)
		if bucket.adjustment {
			continue
		}
		vendorBucket := vendorUsage[bucket.vendorID]
		if vendorBucket == nil {
			vendorBucket = &billBucket{
				vendorID:   bucket.vendorID,
				vendorName: bucket.vendorName,
				vendorIcon: bucket.vendorIcon,
			}
			vendorUsage[bucket.vendorID] = vendorBucket
		}
		vendorBucket.usage.merge(bucket.usage)
	}
	vendors := make([]BillVendorTotal, 0, len(vendorUsage))
	for _, bucket := range vendorUsage {
		vendors = append(vendors, BillVendorTotal{
			VendorID:    bucket.vendorID,
			VendorName:  bucket.vendorName,
			VendorIcon:  bucket.vendorIcon,
			BillMetrics: bucket.usage.metrics(),
		})
	}
	slices.SortFunc(vendors, func(a, b BillVendorTotal) int {
		if a.ActualQuota != b.ActualQuota {
			if a.ActualQuota > b.ActualQuota {
				return -1
			}
			return 1
		}
		return strings.Compare(a.VendorName, b.VendorName)
	})
	return UserBill{
		Total:   total.metrics(),
		Vendors: vendors,
		Rows:    rows,
		Facets:  facets,
	}
}

func billBucketMatches(bucket *billBucket, filter BillFilter) bool {
	if filter.VendorID != nil && bucket.vendorID != *filter.VendorID {
		return false
	}
	if filter.ModelName != "" && bucket.modelName != filter.ModelName {
		return false
	}
	if filter.TokenName != "" && bucket.tokenName != filter.TokenName {
		return false
	}
	if filter.Group != "" && bucket.group != filter.Group {
		return false
	}
	return true
}

func billFacets(buckets []*billBucket) BillFacets {
	vendors := map[int]BillFacetVendor{}
	models := map[string]struct{}{}
	tokens := map[string]struct{}{}
	groups := map[string]struct{}{}
	for _, bucket := range buckets {
		if bucket.adjustment {
			continue
		}
		vendors[bucket.vendorID] = BillFacetVendor{
			ID:   bucket.vendorID,
			Name: bucket.vendorName,
			Icon: bucket.vendorIcon,
		}
		if bucket.modelName != "" {
			models[bucket.modelName] = struct{}{}
		}
		if bucket.tokenName != "" {
			tokens[bucket.tokenName] = struct{}{}
		}
		if bucket.group != "" {
			groups[bucket.group] = struct{}{}
		}
	}
	facets := BillFacets{
		Vendors: make([]BillFacetVendor, 0, len(vendors)),
		Models:  sortedKeys(models),
		Tokens:  sortedKeys(tokens),
		Groups:  sortedKeys(groups),
	}
	for _, vendor := range vendors {
		facets.Vendors = append(facets.Vendors, vendor)
	}
	slices.SortFunc(facets.Vendors, func(a, b BillFacetVendor) int {
		if a.ID == 0 && b.ID != 0 {
			return 1
		}
		if b.ID == 0 && a.ID != 0 {
			return -1
		}
		return strings.Compare(a.Name, b.Name)
	})
	return facets
}

func sortedKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

// BuildUserBill groups log rows into a statement. Vendors are matched by the current model name.
func BuildUserBill(sources []BillSource, vendors map[string]billVendorRef, filter BillFilter) UserBill {
	accumulator := newBillAccumulator()
	for _, source := range sources {
		accumulator.add(source, vendors)
	}
	return accumulator.finish(filter)
}

// QueryUserBill reads one user's consume and refund logs and aggregates them in process.
func QueryUserBill(ctx context.Context, userID int, start int64, end int64, filter BillFilter) (UserBill, error) {
	vendors, err := loadBillVendors()
	if err != nil {
		return UserBill{}, err
	}
	accumulator := newBillAccumulator()
	err = scanUserBillSources(ctx, userID, start, end, func(source BillSource) {
		accumulator.add(source, vendors)
	})
	if err != nil {
		return UserBill{}, err
	}
	return accumulator.finish(filter), nil
}

func loadBillVendors() (map[string]billVendorRef, error) {
	type vendorRow struct {
		ModelName string `gorm:"column:model_name"`
		VendorID  int    `gorm:"column:vendor_id"`
		Name      string `gorm:"column:name"`
		Icon      string `gorm:"column:icon"`
	}
	var rows []vendorRow
	err := DB.Table("models").
		Select("models.model_name as model_name, models.vendor_id as vendor_id, vendors.name as name, vendors.icon as icon").
		Joins("LEFT JOIN vendors ON vendors.id = models.vendor_id AND vendors.deleted_at IS NULL").
		Where("models.deleted_at IS NULL").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	vendors := make(map[string]billVendorRef, len(rows))
	for _, row := range rows {
		if row.VendorID == 0 {
			continue
		}
		vendors[row.ModelName] = billVendorRef{ID: row.VendorID, Name: row.Name, Icon: row.Icon}
	}
	return vendors, nil
}

type billScanRow struct {
	Type             int
	ModelName        string
	TokenName        string
	TokenId          int
	Group            string
	Quota            int
	PromptTokens     int
	CompletionTokens int
	Other            string
}

func scanUserBillSources(ctx context.Context, userID int, start int64, end int64, fn func(BillSource)) error {
	rows, err := LOG_DB.WithContext(ctx).Model(&Log{}).
		Select("type", "model_name", "token_name", "token_id", "group", "quota", "prompt_tokens", "completion_tokens", "other").
		Where("user_id = ? AND created_at >= ? AND created_at <= ? AND type IN ?", userID, start, end, []int{LogTypeConsume, LogTypeRefund}).
		Rows()
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var row billScanRow
		if err := LOG_DB.ScanRows(rows, &row); err != nil {
			return err
		}
		fn(BillSource{
			Type:             row.Type,
			ModelName:        row.ModelName,
			TokenName:        row.TokenName,
			TokenID:          row.TokenId,
			Group:            row.Group,
			Quota:            row.Quota,
			PromptTokens:     row.PromptTokens,
			CompletionTokens: row.CompletionTokens,
			Other:            row.Other,
		})
	}
	return rows.Err()
}
