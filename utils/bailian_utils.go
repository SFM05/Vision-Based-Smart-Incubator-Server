package utils

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
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

	client := &http.Client{}
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
						Text: "你现在是一个微生物领域的专家。请你根据菌落的形状、边缘、表面质地、颜色等维度，判断这张图片中是否有杂菌。在回复的开始按\u201c污染可能性：低/中/高\\n\u201d回复，并陈述你的理由。",
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
		return "", ""
	}

	req, err := http.NewRequest("POST", "https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		slog.Error(fmt.Sprintf("Fail to create request: %v", err))
		return "", ""
	}
	apiKey := os.Getenv("DASHSCOPE_API_KEY")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		slog.Error(fmt.Sprintf("Send request failed: %v", err))
		return "", ""
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

func UploadSucess(uuid string, timestamp time.Time, plateid int) {
	img_path := uuid + "/" +
		strconv.Itoa(plateid) + "/" +
		timestamp.Format("20060102-150405") + ".bmp"

	model_name := os.Getenv("MODEL_NAME")
	_, content := BailianInference(img_path, model_name)

	client := InitClient()
	table_name := os.Getenv("COLONY_TABLE_NAME")
	measurement_name := os.Getenv("COLONY_MEASURE_NAME")

	timeseriesKey := tablestore.NewTimeseriesKey()
	timeseriesKey.SetMeasurementName(measurement_name)
	timeseriesKey.SetDataSource(uuid)
	timeseriesKey.AddTag("plate_id", strconv.Itoa(plateid))

	// Use truncated timestamp to match write path
	truncatedUs := timestamp.UnixMicro() / 1e6 * 1e6
	getTimeseriesDataRequest := tablestore.NewGetTimeseriesDataRequest(table_name)
	getTimeseriesDataRequest.SetTimeseriesKey(timeseriesKey)
	getTimeseriesDataRequest.SetTimeRange(truncatedUs, truncatedUs+1)
	getTimeseriesDataRequest.SetLimit(-1)

	getTimeseriesResp, err := client.GetTimeseriesData(getTimeseriesDataRequest)
	if err != nil {
		slog.Error(fmt.Sprintf("Fetch table content failed: %v", err))
		return
	}

	for i := 0; i < len(getTimeseriesResp.GetRows()); i++ {
		if getTimeseriesResp.GetRows()[i].GetTimeInus() == truncatedUs {
			rows := getTimeseriesResp.GetRows()[i].GetFieldsMap()

			timeseriesKey := tablestore.NewTimeseriesKey()
			timeseriesKey.SetMeasurementName(measurement_name)
			timeseriesKey.SetDataSource(uuid)
			timeseriesKey.AddTag("plate_id", strconv.Itoa(plateid))

			timeseriesRow := tablestore.NewTimeseriesRow(timeseriesKey)
			timeseriesRow.SetTimeInus(truncatedUs)

			for key, value := range rows {
				timeseriesRow.AddField(key, value)
			}
			timeseriesRow.AddField("reply",
				tablestore.NewColumnValue(tablestore.ColumnType_STRING, content))

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
