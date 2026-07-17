package utils

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/aliyun/aliyun-tablestore-go-sdk/tablestore"
)

func InitClient() *tablestore.TimeseriesClient {
	instanceName := os.Getenv("TABLE_INSTANCE_NAME")
	endpoint := os.Getenv("TABLE_ENDPOINT")
	accessKeyId := os.Getenv("TABLESTORE_ACCESS_KEY_ID")
	accessKeySecret := os.Getenv("TABLESTORE_ACCESS_KEY_SECRET")

	client := tablestore.NewTimeseriesClient(endpoint, instanceName, accessKeyId, accessKeySecret)
	return client
}

func loadLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		slog.Warn(fmt.Sprintf("Failed to load Asia/Shanghai timezone, falling back to UTC: %v", err))
		return time.UTC
	}
	return loc
}

// RecordEnvData 记录温湿度数据
func OnDataReceived(uuid string, payload string) {
	client := InitClient()

	// {"timestamp":string, "temp":float, "hum":float}
	var json_result struct {
		TimeStamp string  `json:"timestamp"`
		Temp      float64 `json:"temp"`
		Hum       float64 `json:"hum"`
	}
	err := json.Unmarshal([]byte(payload), &json_result)
	if err != nil {
		slog.Error(fmt.Sprintf("Encounter error when decoding json: %v", err))
		slog.Error(fmt.Sprintf("    Original message: %s", payload))
		return
	}

	loc := loadLocation()
	timestamp, err := time.ParseInLocation("20060102-150405", json_result.TimeStamp, loc)
	if err != nil {
		slog.Warn(fmt.Sprintf("Time parse fail: %v", err))
		slog.Warn(fmt.Sprintf("    Original time: %s", json_result.TimeStamp))
		slog.Warn("Using server time instead")
		timestamp = time.Now().In(loc)
	}

	table_name := os.Getenv("ENV_TABLE_NAME")
	measurement_name := os.Getenv("ENV_MEASURE_NAME")

	timeseriesKey := tablestore.NewTimeseriesKey()
	timeseriesKey.SetMeasurementName(measurement_name)
	timeseriesKey.SetDataSource(uuid)

	timeseriesRow := tablestore.NewTimeseriesRow(timeseriesKey)
	timeseriesRow.SetTimeInus(timestamp.UnixMicro() / 1e6 * 1e6)
	timeseriesRow.AddField("temperature",
		tablestore.NewColumnValue(tablestore.ColumnType_DOUBLE, json_result.Temp))
	timeseriesRow.AddField("humidity",
		tablestore.NewColumnValue(tablestore.ColumnType_DOUBLE, json_result.Hum))

	putTimeseriesDataRequest := tablestore.NewPutTimeseriesDataRequest(table_name)
	putTimeseriesDataRequest.AddTimeseriesRows(timeseriesRow)

	_, err = client.PutTimeseriesData(putTimeseriesDataRequest)
	if err != nil {
		slog.Error(fmt.Sprintf("Fail to write into the table: %v", err))
		return
	}

	slog.Info("Success to write environment record into the table")
}

// RecordColonyData 记录图片和详细结果的存储路径
func RecordColonyData(uuid string,
	plate_id int,
	timestamp time.Time,
	img_path string,
	txt_path string,
	number int) {
	client := InitClient()

	table_name := os.Getenv("COLONY_TABLE_NAME")
	measurement_name := os.Getenv("COLONY_MEASURE_NAME")

	timeseriesKey := tablestore.NewTimeseriesKey()
	timeseriesKey.SetMeasurementName(measurement_name)
	timeseriesKey.SetDataSource(uuid)
	timeseriesKey.AddTag("plate_id", strconv.Itoa(plate_id))

	timeseriesRow := tablestore.NewTimeseriesRow(timeseriesKey)
	timeseriesRow.SetTimeInus(timestamp.UnixMicro() / 1e6 * 1e6)
	timeseriesRow.AddField("image",
		tablestore.NewColumnValue(tablestore.ColumnType_STRING, img_path))
	timeseriesRow.AddField("detail",
		tablestore.NewColumnValue(tablestore.ColumnType_STRING, txt_path))
	timeseriesRow.AddField("number",
		tablestore.NewColumnValue(tablestore.ColumnType_INTEGER, number))

	putTimeseriesDataRequest := tablestore.NewPutTimeseriesDataRequest(table_name)
	putTimeseriesDataRequest.AddTimeseriesRows(timeseriesRow)

	_, err := client.PutTimeseriesData(putTimeseriesDataRequest)
	if err != nil {
		slog.Error(fmt.Sprintf("Fail to write into the table: %v", err))
		return
	}
	slog.Info("Success to write colony data into the table")
}
