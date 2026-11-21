package model

import (
	"slices"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

func IsChannelEnabledForGroupModel(group string, modelName string, channelID int) bool {
	if group == "" || modelName == "" || channelID <= 0 {
		return false
	}
	if !common.MemoryCacheEnabled {
		return isChannelEnabledForGroupModelDB(group, modelName, channelID)
	}

	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	if group2model2channels == nil {
		return false
	}

	if isChannelIDInList(group2model2channels[group][modelName], channelID) {
		return true
	}
	normalized := ratio_setting.RoutingMatchModelName(modelName)
	if normalized != "" && normalized != modelName {
		return isChannelIDInList(group2model2channels[group][normalized], channelID)
	}
	return false
}

func IsChannelEnabledForAnyGroupModel(groups []string, modelName string, channelID int) bool {
	if len(groups) == 0 {
		return false
	}
	for _, g := range groups {
		if IsChannelEnabledForGroupModel(g, modelName, channelID) {
			return true
		}
	}
	return false
}

func isChannelEnabledForGroupModelDB(group string, modelName string, channelID int) bool {
	var count int64
	err := DB.Model(&Ability{}).
		Where(commonGroupCol+" = ? and model = ? and channel_id = ? and enabled = ?", group, modelName, channelID, true).
		Count(&count).Error
	if err == nil && count > 0 {
		return true
	}
	normalized := ratio_setting.RoutingMatchModelName(modelName)
	if normalized == "" || normalized == modelName {
		return false
	}
	count = 0
	err = DB.Model(&Ability{}).
		Where(commonGroupCol+" = ? and model = ? and channel_id = ? and enabled = ?", group, normalized, channelID, true).
		Count(&count).Error
	return err == nil && count > 0
}

func isChannelIDInList(list []int, channelID int) bool {
	return slices.Contains(list, channelID)
}

// GroupHasChannelForModel reports whether group has any channel configured for
// modelName or its routing name, including channels that are currently disabled.
// A false result is the unknown-model case. Selection errors and channels that
// exist but cannot be used are a separate outcome.
func GroupHasChannelForModel(group, modelName string) (bool, error) {
	if group == "" || modelName == "" {
		return false, nil
	}
	names := []string{modelName}
	if normalized := ratio_setting.RoutingMatchModelName(modelName); normalized != "" && normalized != modelName {
		names = append(names, normalized)
	}
	if !common.MemoryCacheEnabled {
		var count int64
		err := DB.Model(&Ability{}).
			Where(commonGroupCol+" = ? AND model IN ?", group, names).
			Count(&count).Error
		return count > 0, err
	}

	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()
	for _, channel := range channelsIDM {
		if channel == nil || !slices.Contains(channel.GetGroups(), group) {
			continue
		}
		for _, configured := range channel.GetModels() {
			if slices.Contains(names, configured) {
				return true, nil
			}
		}
	}
	return false, nil
}
