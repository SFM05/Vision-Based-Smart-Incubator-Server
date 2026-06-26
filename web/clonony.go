package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"mqtt_listener/utils"
	"strconv"
	"time"

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"
	"github.com/aliyun/aliyun-tablestore-go-sdk/tablestore"
)

type FileMetaData struct {
	Success bool   `json:"success"`
	Mesaage string `json:"message,omitempty"`
	URL     string `json:"url,omitempty"`
}
type ColonyMetaData struct {
	Timestamp string       `json:"timestamp"`
	Number    int64        `json:"number"`
	Image     FileMetaData `json:"image"`
	Record    FileMetaData `json:"record"`
}
type ColonyResponse struct {
	Sucess     bool             `json:"success"`
	Message    string           `json:"message,omitempty"`
	ColonyData []ColonyMetaData `json:"colony,omitempty"`
}

func getFileMetaData(path string) FileMetaData {
	// 加载默认配置并设置凭证提供者和区域
	cfg := oss.LoadDefaultConfig().
		WithCredentialsProvider(credentials.NewEnvironmentVariableCredentialsProvider()).
		WithRegion("cn-hangzhou")

	// 创建OSS客户端
	client := oss.NewClient(cfg)

	data := FileMetaData{}
	existed, err := client.IsObjectExist(context.TODO(), "embedded-comptition", path)
	if err != nil {
		data.Success = false
		data.Mesaage = err.Error()
	} else {
		if existed {
			result, err := client.Presign(context.TODO(), &oss.GetObjectRequest{
				Bucket: oss.Ptr("embedded-comptition"),
				Key:    oss.Ptr(path),
			},
				oss.PresignExpires(10*time.Minute),
			)
			if err != nil {
				data.Success = false
				data.Mesaage = err.Error()
			} else {
				data.Success = true
				data.URL = result.URL
			}
		} else {
			data.Success = false
			data.Mesaage = "Not existed."
		}
	}

	return data
}

func GetColony(uuid string, plateid int, start time.Time, end time.Time) string {
	client := utils.InitClient()
	// 构造待查询时间线的 timeseriesKey。
	timeseriesKey := tablestore.NewTimeseriesKey()
	timeseriesKey.SetMeasurementName("device_colony")
	timeseriesKey.SetDataSource(uuid)
	timeseriesKey.AddTag("plate_id", strconv.Itoa(plateid))

	// 构造查询请求。
	getTimeseriesDataRequest := tablestore.NewGetTimeseriesDataRequest("colony")
	getTimeseriesDataRequest.SetTimeseriesKey(timeseriesKey)
	getTimeseriesDataRequest.SetTimeRange(start.UnixMicro(), end.UnixMicro()) // 指定查询时间范围。
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
		// TODO
	}

	response := ColonyResponse{
		Sucess: true,
	}

	for i := 0; i < len(getTimeseriesResp.GetRows()); i++ {
		timestamp := time.UnixMicro(getTimeseriesResp.GetRows()[i].GetTimeInus())

		rows := getTimeseriesResp.GetRows()[i].GetFieldsMap()

		image_path := rows["image"].Value.(string)
		image := getFileMetaData(image_path)
		record_path := rows["detail"].Value.(string)
		record := getFileMetaData(record_path)

		data := ColonyMetaData{
			Timestamp: timestamp.Format("2006-01-02T15:04:05Z"),
			Number:    rows["number"].Value.(int64),
			Image:     image,
			Record:    record,
		}

		response.ColonyData = append(response.ColonyData, data)
	}

	// 禁用转义
	json_data := &bytes.Buffer{}
	encoder := json.NewEncoder(json_data)
	encoder.SetEscapeHTML(false)
	encoder.Encode(response)

	return string(json_data.String())
}
