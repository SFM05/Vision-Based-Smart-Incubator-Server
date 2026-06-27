package web

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
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
	// yourInstanceName 填写您的实例名称
	instanceName := os.Getenv("TABLE_INSTANCE_NAME")
	// yourEndpoint 填写您的实例访问地址
	endpoint := os.Getenv("TABLE_ENDPOINT")
	// 获取环境变量里的 AccessKey ID 和 AccessKey Secret
	accessKeyId := os.Getenv("TABLESTORE_ACCESS_KEY_ID")
	accessKeySecret := os.Getenv("TABLESTORE_ACCESS_KEY_SECRET")

	// 初始化表格存储客户端
	client := tablestore.NewTimeseriesClient(endpoint, instanceName, accessKeyId, accessKeySecret)

	table_name := os.Getenv("ENV_TABLE_NAME")
	measurement_name := os.Getenv("ENV_MEATURE_NAME")

	// 构造待查询时间线的 timeseriesKey。
	timeseriesKey := tablestore.NewTimeseriesKey()
	timeseriesKey.SetMeasurementName(measurement_name)
	timeseriesKey.SetDataSource(uuid)

	// 构造查询请求。
	getTimeseriesDataRequest := tablestore.NewGetTimeseriesDataRequest(table_name)
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
