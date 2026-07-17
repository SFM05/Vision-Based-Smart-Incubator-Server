package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"github.com/aliyun/aliyun-tablestore-go-sdk/tablestore"
)

type FileMetaData struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	URL     string `json:"url,omitempty"`
	Key     string `json:"key,omitempty"`
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
	bucket_name := os.Getenv("BUCKET_NAME")
	data := FileMetaData{}
	if object_name == "" {
		data.Success = false
		data.Message = "empty object name"
		return data
	}
	data.Key = object_name

	exists, err := getOSSClient().IsObjectExist(context.TODO(), bucket_name, object_name)
	if err != nil {
		data.Success = false
		data.Message = err.Error()
		return data
	}
	if !exists {
		data.Success = false
		data.Message = "object not found"
		return data
	}

	result, err := getOSSClient().Presign(context.TODO(), &oss.GetObjectRequest{
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

func GetRecordText(objectName string) (string, error) {
	bucketName := os.Getenv("BUCKET_NAME")
	result, err := getOSSClient().GetObject(context.TODO(), &oss.GetObjectRequest{
		Bucket: oss.Ptr(bucketName),
		Key:    oss.Ptr(objectName),
	})
	if err != nil {
		return "", err
	}
	defer result.Body.Close()

	body, err := io.ReadAll(result.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
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
	client := getTimeseriesClient()

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
	imagePaths := []string{}
	recordPaths := []string{}

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

		data := ColonyMetaData{
			Timestamp: timestamp.UTC().Format(time.RFC3339),
			Number:    number,
			Reply:     reply,
			UserBoxes: userBoxes,
		}

		response.ColonyData = append(response.ColonyData, data)
		imagePaths = append(imagePaths, image_path)
		recordPaths = append(recordPaths, record_path)
	}

	var wg sync.WaitGroup
	limit := make(chan struct{}, 8)
	for i := range response.ColonyData {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			limit <- struct{}{}
			defer func() { <-limit }()

			response.ColonyData[i].Image = getFileMetaData(imagePaths[i], 10*time.Minute)
			response.ColonyData[i].Record = getFileMetaData(recordPaths[i], 10*time.Minute)
		}()
	}
	wg.Wait()

	json_data := &bytes.Buffer{}
	encoder := json.NewEncoder(json_data)
	encoder.SetEscapeHTML(false)
	encoder.Encode(response)

	return string(json_data.String())
}
