package web

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"mqtt_listener/utils"
	"time"

	"github.com/aliyun/aliyun-tablestore-go-sdk/tablestore"
)

type EnvMetaData struct {
	Timestamp string  `json:"timestamp"`
	Temp      float64 `json:"temp"`
	Hum       float64 `json:"hum"`
}
type EnvResponse struct {
	Sucess  bool          `json:"sucess"`
	Message string        `json:"message,omitempty"`
	EnvData []EnvMetaData `json:"env,omitempty"`
}

// GetEnv 获取网页需要的温湿度数据
func GetEnv(uuid string, start time.Time, end time.Time) string {
	client := utils.InitClient()
	// 构造待查询时间线的 timeseriesKey。
	timeseriesKey := tablestore.NewTimeseriesKey()
	timeseriesKey.SetMeasurementName("device_env")
	timeseriesKey.SetDataSource(uuid)

	// 构造查询请求。
	getTimeseriesDataRequest := tablestore.NewGetTimeseriesDataRequest("env")
	getTimeseriesDataRequest.SetTimeseriesKey(timeseriesKey)
	getTimeseriesDataRequest.SetTimeRange(start.UnixMicro(), end.UnixMicro()) // 指定查询时间范围。
	getTimeseriesDataRequest.SetLimit(-1)

	getTimeseriesResp, err := client.GetTimeseriesData(getTimeseriesDataRequest)
	if err != nil {
		slog.Error(fmt.Sprintf("Fetch table content failed: %v", err))
		response := EnvResponse{
			Sucess:  false,
			Message: err.Error(),
		}
		json_data, _ := json.Marshal(response)
		return string(json_data)
		// TODO
	}

	response := EnvResponse{
		Sucess: true,
	}

	for i := 0; i < len(getTimeseriesResp.GetRows()); i++ {
		timestamp := time.UnixMicro(getTimeseriesResp.GetRows()[i].GetTimeInus())

		rows := getTimeseriesResp.GetRows()[i].GetFieldsMap()

		data := EnvMetaData{
			Timestamp: timestamp.Format("2006-01-02T15:04:05Z"),
			Temp:      rows["temperature"].Value.(float64),
			Hum:       rows["humidity"].Value.(float64),
		}

		response.EnvData = append(response.EnvData, data)
	}

	json_data, _ := json.Marshal(response)

	return string(json_data)
}
