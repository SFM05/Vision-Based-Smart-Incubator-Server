package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"
	"github.com/aliyun/aliyun-tablestore-go-sdk/tablestore"
)

type FileMetaData struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	URL     string `json:"url,omitempty"`
}
type ColonyMetaData struct {
	Timestamp string       `json:"timestamp"`
	Number    int64        `json:"number"`
	Image     FileMetaData `json:"image"`
	Record    FileMetaData `json:"record"`
	Reply     string       `json:"reply,omitempty"`
	UserBoxes string       `json:"user_boxes,omitempty"`
}
type ColonyResponse struct {
	Sucess     bool             `json:"success"`
	Message    string           `json:"message,omitempty"`
	ColonyData []ColonyMetaData `json:"colony,omitempty"`
}

func getFileMetaData(object_name string, expire_time time.Duration) FileMetaData {
	region := os.Getenv("REGION")
	bucket_name := os.Getenv("BUCKET_NAME")

	cfg := oss.LoadDefaultConfig().
		WithCredentialsProvider(credentials.NewEnvironmentVariableCredentialsProvider()).
		WithRegion(region)

	client := oss.NewClient(cfg)

	data := FileMetaData{}
	existed, err := client.IsObjectExist(context.TODO(), bucket_name, object_name)
	if err != nil {
		data.Success = false
		data.Message = err.Error()
		return data
	}
	if !existed {
		data.Success = false
		data.Message = "Not existed."
		return data
	}

	result, err := client.Presign(context.TODO(), &oss.GetObjectRequest{
		Bucket: oss.Ptr(bucket_name),
		Key:    oss.Ptr(object_name),
	},
		oss.PresignExpires(expire_time),
	)
	if err != nil {
		data.Success = false
		data.Message = err.Error()
		return data
	}
	data.Success = true
	data.URL = result.URL
	return data
}

func safeString(fields map[string]*tablestore.ColumnValue, key string) (string, bool) {
	f, ok := fields[key]
	if !ok || f == nil {
		return "", false
	}
	v, ok := f.Value.(string)
	if !ok {
		return "", false
	}
	return v, true
}

func safeInt64(fields map[string]*tablestore.ColumnValue, key string) (int64, bool) {
	f, ok := fields[key]
	if !ok || f == nil {
		return 0, false
	}
	v, ok := f.Value.(int64)
	if !ok {
		return 0, false
	}
	return v, true
}

func GetColony(uuid string, plateid int, start time.Time, end time.Time) string {
	instanceName := os.Getenv("TABLE_INSTANCE_NAME")
	endpoint := os.Getenv("TABLE_ENDPOINT")
	accessKeyId := os.Getenv("TABLESTORE_ACCESS_KEY_ID")
	accessKeySecret := os.Getenv("TABLESTORE_ACCESS_KEY_SECRET")

	client := tablestore.NewTimeseriesClient(endpoint, instanceName, accessKeyId, accessKeySecret)

	table_name := os.Getenv("COLONY_TABLE_NAME")
	measurement_name := os.Getenv("COLONY_MEASURE_NAME")

	timeseriesKey := tablestore.NewTimeseriesKey()
	timeseriesKey.SetMeasurementName(measurement_name)
	timeseriesKey.SetDataSource(uuid)
	timeseriesKey.AddTag("plate_id", strconv.Itoa(plateid))

	getTimeseriesDataRequest := tablestore.NewGetTimeseriesDataRequest(table_name)
	getTimeseriesDataRequest.SetTimeseriesKey(timeseriesKey)
	getTimeseriesDataRequest.SetTimeRange(start.UnixMicro(), end.UnixMicro())
	getTimeseriesDataRequest.SetLimit(-1)

	getTimeseriesResp, err := client.GetTimeseriesData(getTimeseriesDataRequest)
	if err != nil {
		slog.Error(fmt.Sprintf("Fetch table content failed: %v", err))
		response := ColonyResponse{
			Sucess:  false,
			Message: err.Error(),
		}
		json_data, _ := json.Marshal(response)
		return string(json_data)
	}

	response := ColonyResponse{
		Sucess: true,
	}

	for i := 0; i < len(getTimeseriesResp.GetRows()); i++ {
		timestamp := time.UnixMicro(getTimeseriesResp.GetRows()[i].GetTimeInus())
		rows := getTimeseriesResp.GetRows()[i].GetFieldsMap()

		image_path, imgOk := safeString(rows, "image")
		record_path, recOk := safeString(rows, "detail")
		number, numOk := safeInt64(rows, "number")
		if !imgOk || !recOk || !numOk {
			slog.Warn(fmt.Sprintf("Skipping colony row with missing fields at %v", timestamp))
			continue
		}

		reply, _ := safeString(rows, "reply")
		userBoxes, _ := safeString(rows, "user_boxes")

		image := getFileMetaData(image_path, 10*time.Minute)
		record := getFileMetaData(record_path, 10*time.Minute)

		data := ColonyMetaData{
			Timestamp: timestamp.UTC().Format(time.RFC3339),
			Number:    number,
			Image:     image,
			Record:    record,
			Reply:     reply,
			UserBoxes: userBoxes,
		}

		response.ColonyData = append(response.ColonyData, data)
	}

	json_data := &bytes.Buffer{}
	encoder := json.NewEncoder(json_data)
	encoder.SetEscapeHTML(false)
	encoder.Encode(response)

	return string(json_data.String())
}
