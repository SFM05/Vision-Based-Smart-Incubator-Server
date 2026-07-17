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

func safeFloat64(fields map[string]*tablestore.ColumnValue, key string) (float64, bool) {
	f, ok := fields[key]
	if !ok || f == nil {
		return 0, false
	}
	v, ok := f.Value.(float64)
	if !ok {
		return 0, false
	}
	return v, true
}

// GetEnv 获取网页需要的温湿度数据
func GetEnv(uuid string, start time.Time, end time.Time) string {
	client := getTimeseriesClient()

	table_name := os.Getenv("ENV_TABLE_NAME")
	measurement_name := os.Getenv("ENV_MEASURE_NAME")

	timeseriesKey := tablestore.NewTimeseriesKey()
	timeseriesKey.SetMeasurementName(measurement_name)
	timeseriesKey.SetDataSource(uuid)

	getTimeseriesDataRequest := tablestore.NewGetTimeseriesDataRequest(table_name)
	getTimeseriesDataRequest.SetTimeseriesKey(timeseriesKey)
	getTimeseriesDataRequest.SetTimeRange(start.UnixMicro(), end.UnixMicro())
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
	}

	response := EnvResponse{
		Sucess: true,
	}

	for i := 0; i < len(getTimeseriesResp.GetRows()); i++ {
		timestamp := time.UnixMicro(getTimeseriesResp.GetRows()[i].GetTimeInus())
		rows := getTimeseriesResp.GetRows()[i].GetFieldsMap()

		temp, tempOk := safeFloat64(rows, "temperature")
		hum, humOk := safeFloat64(rows, "humidity")
		if !tempOk || !humOk {
			slog.Warn(fmt.Sprintf("Skipping row with missing or invalid fields at %v", timestamp))
			continue
		}

		data := EnvMetaData{
			Timestamp: timestamp.UTC().Format(time.RFC3339),
			Temp:      temp,
			Hum:       hum,
		}

		response.EnvData = append(response.EnvData, data)
	}

	json_data, _ := json.Marshal(response)

	return string(json_data)
}
