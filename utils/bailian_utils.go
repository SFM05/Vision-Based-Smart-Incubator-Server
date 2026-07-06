package utils

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aliyun/aliyun-tablestore-go-sdk/tablestore"
)

type ImageURL struct {
	URL    string `json:"url,omitempty"`
	Detail string `json:"detail,omitempty"`
}
type ContentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}
type StreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}
type ChatMessage struct {
	Role    string        `json:"role"`
	Content []ContentPart `json:"content"`
}
type ChatRequest struct {
	Model          string         `json:"model"`
	Messages       []ChatMessage  `json:"messages"`
	Stream         bool           `json:"stream"`
	StreamOptions  *StreamOptions `json:"stream_options,omitempty"`
	EnableThinking bool           `json:"enable_thinking,omitempty"`
}

// 进行AI推理
func BailianInference(img_path string, model_name string) (string, string) {
	img_url, err := signDownloadURL(img_path, 10*time.Minute)
	if err != nil {
		slog.Error(fmt.Sprintf("Fail to sign URL: %v", err))
		return "", ""
	}

	// 创建 HTTP 客户端
	client := &http.Client{}
	// 构建请求体
	requestBody := ChatRequest{
		Model: model_name,
		Messages: []ChatMessage{
			{
				Role: "user",
				Content: []ContentPart{
					{
						Type: "image_url",
						ImageURL: &ImageURL{
							URL: img_url,
						},
					},
					{
						Type: "text",
						Text: "你现在是一个微生物领域的专家。请你根据菌落的形状、边缘、表面质地、颜色等维度，判断这张图片中是否有杂菌。在回复的开始按“污染可能性：低/中/高\\n”回复，并陈述你的理由。",
					},
				},
			},
		},
		Stream:         true,
		StreamOptions:  &StreamOptions{IncludeUsage: true},
		EnableThinking: false,
	}
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		slog.Error(fmt.Sprintf("Fail to construct request body: %v", err))
	}

	// 创建 POST 请求
	req, err := http.NewRequest("POST", "https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Fatal(err)
	}
	// 设置请求头
	apiKey := os.Getenv("DASHSCOPE_API_KEY")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		slog.Error(fmt.Sprintf("Send request failed: %v", err))
	}
	defer resp.Body.Close()

	// 流式输出答案
	reasoning_content := ""
	content := ""
	scanner := bufio.NewScanner(resp.Body)
	is_thinking := false
	is_content := false
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content          string `json:"content"`
					ReasoningContent string `json:"reasoning_content"`
				} `json:"delta"`
				FinishReason *string `json:"finish_reason"`
			} `json:"choices"`
		}
		json.Unmarshal([]byte(data), &chunk)

		for _, c := range chunk.Choices {
			if c.Delta.ReasoningContent != "" {
				if !is_thinking {
					fmt.Print("[Thinking] ")
					is_thinking = true
				}

				fmt.Print(c.Delta.ReasoningContent)
				reasoning_content += c.Delta.ReasoningContent
			}
			if c.Delta.Content != "" {
				if !is_content {
					fmt.Print("\n[Reply] ")
					is_content = true
				}

				fmt.Print(c.Delta.Content)
				content += c.Delta.Content
			}
		}
	}

	return reasoning_content, content
}

// OnUploadSucess 回调函数，处理上传成功的图片
func OnUploadSucess(uuid string, payload string) {
	// {"timestamp":string, "plateid":int}
	var json_result struct {
		Timestamp string `json:"timestamp"`
		PlateID   int    `json:"plateid"`
	}
	err := json.Unmarshal([]byte(payload), &json_result)
	if err != nil {
		slog.Error(fmt.Sprintf("Encounter error when decoding json: %v", err))
		slog.Error(fmt.Sprintf("    Original message: %s", payload))
		return
	}

	// 解析时间
	loc, _ := time.LoadLocation("Asia/Shanghai")
	timestamp, err := time.ParseInLocation("2006-01-02 15:04:05", json_result.Timestamp, loc)
	if err != nil {
		slog.Error(fmt.Sprintf("Time parse fail: %v", err))
		slog.Error(fmt.Sprintf("    Original time: %s", json_result.Timestamp))
		return
	}

	// 生成图片的预签名URL
	img_path := uuid + "/" +
		strconv.Itoa(json_result.PlateID) + "/" +
		timestamp.Format("20060102-150405") + ".jpg"

	model_name := os.Getenv("MODEL_NAME")
	_, content := BailianInference(img_path, model_name)

	client := InitClient()
	table_name := os.Getenv("COLONY_TABLE_NAME")
	measurement_name := os.Getenv("COLONY_MEASURE_NAME")

	// 构造待查询时间线的 timeseriesKey。
	timeseriesKey := tablestore.NewTimeseriesKey()
	timeseriesKey.SetMeasurementName(measurement_name)
	timeseriesKey.SetDataSource(uuid)
	timeseriesKey.AddTag("plate_id", strconv.Itoa(json_result.PlateID))

	// 构造查询请求。
	getTimeseriesDataRequest := tablestore.NewGetTimeseriesDataRequest(table_name)
	getTimeseriesDataRequest.SetTimeseriesKey(timeseriesKey)
	getTimeseriesDataRequest.SetTimeRange(timestamp.UnixMicro(), timestamp.UnixMicro()+1) // 指定查询时间范围。
	getTimeseriesDataRequest.SetLimit(-1)

	getTimeseriesResp, err := client.GetTimeseriesData(getTimeseriesDataRequest)
	if err != nil {
		slog.Error(fmt.Sprintf("Fetch table content failed: %v", err))
		return
	}

	for i := 0; i < len(getTimeseriesResp.GetRows()); i++ {
		if getTimeseriesResp.GetRows()[i].GetTimeInus() == timestamp.UnixMicro() {
			rows := getTimeseriesResp.GetRows()[i].GetFieldsMap()

			// 构造时序数据行 timeseriesRow。
			// timeseriesKey 标识时间线：度量名称、数据源主机和标签。
			timeseriesKey := tablestore.NewTimeseriesKey()
			timeseriesKey.SetMeasurementName(measurement_name)
			timeseriesKey.SetDataSource(uuid)
			timeseriesKey.AddTag("plate_id", strconv.Itoa(json_result.PlateID))

			// timeseriesRow 在 timeseriesKey 的基础上关联时间戳和字段值。
			timeseriesRow := tablestore.NewTimeseriesRow(timeseriesKey)
			timeseriesRow.SetTimeInus(timestamp.UnixMicro())

			// 把原来的数据写入
			for key, value := range rows {
				timeseriesRow.AddField(key, value)
			}
			timeseriesRow.AddField("reply",
				tablestore.NewColumnValue(tablestore.ColumnType_STRING, content))

			// 构造写入时序数据的请求。
			putTimeseriesDataRequest := tablestore.NewPutTimeseriesDataRequest(table_name)
			putTimeseriesDataRequest.AddTimeseriesRows(timeseriesRow)

			_, err := client.PutTimeseriesData(putTimeseriesDataRequest)
			if err != nil {
				slog.Error(fmt.Sprintf("Fail to write into the table: %v", err))
				return
			}
			slog.Info("Success to write into the table")
		}
	}
}
