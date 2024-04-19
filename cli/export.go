package cli

import (
	"encoding/json"
	"one-api/common"
	"one-api/relay/util"
	"os"
	"sort"
)

func ExportPrices() {
	prices := util.GetPricesList("default")

	if len(prices) == 0 {
		common.SysError("No prices found")
		return
	}

	// Sort prices by ChannelType
	sort.Slice(prices, func(i, j int) bool {
		if prices[i].ChannelType == prices[j].ChannelType {
			return prices[i].Model < prices[j].Model
		}
		return prices[i].ChannelType < prices[j].ChannelType
	})

	// 导出到当前目录下的 prices.json 文件
	file, err := os.Create("prices.json")
	if err != nil {
		common.SysError("Failed to create file: " + err.Error())
		return
	}
	defer file.Close()

	jsonData, err := json.MarshalIndent(prices, "", "  ")
	if err != nil {
		common.SysError("Failed to encode prices: " + err.Error())
		return
	}

	_, err = file.Write(jsonData)
	if err != nil {
		common.SysError("Failed to write to file: " + err.Error())
		return
	}

	common.SysLog("Prices exported to prices.json")
}
