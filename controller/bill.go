package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// GetUserBill aggregates the caller's consume and refund logs into a statement.
func GetUserBill(c *gin.Context) {
	start, end, err := billTimeRange(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	filter, err := billFilterFromQuery(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	bill, err := model.QueryUserBill(c.Request.Context(), c.GetInt("id"), start, end, filter)
	if err != nil {
		common.SysError(common.LogText("failed to query user bill: %s", err.Error()))
		common.ApiErrorT(c, "Failed to load the bill")
		return
	}
	common.ApiSuccess(c, bill)
}

func billTimeRange(c *gin.Context) (int64, int64, error) {
	start, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	end, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	start, err := limitLogTimeRange(start, end)
	if err != nil {
		return 0, 0, err
	}
	if end == 0 || end > common.GetTimestamp() {
		end = common.GetTimestamp()
	}
	return start, end, nil
}

func billFilterFromQuery(c *gin.Context) (model.BillFilter, error) {
	filter := model.BillFilter{
		ModelName: c.Query("model_name"),
		TokenName: c.Query("token_name"),
		Group:     c.Query("group"),
	}
	rawVendor := c.Query("vendor_id")
	if rawVendor == "" {
		return filter, nil
	}
	vendorID, err := strconv.Atoi(rawVendor)
	if err != nil || vendorID < 0 {
		return filter, common.NewMessage("Invalid vendor")
	}
	filter.VendorID = &vendorID
	return filter, nil
}
