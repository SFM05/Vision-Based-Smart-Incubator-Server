package web

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"github.com/aliyun/aliyun-tablestore-go-sdk/tablestore"
)

type AnalyzeResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Reply   string `json:"reply,omitempty"`
}

func signDownloadURL(objectName string, expireTime time.Duration) (string, error) {
	bucketName := os.Getenv("BUCKET_NAME")

	result, err := getOSSClient().Presign(context.TODO(), &oss.GetObjectRequest{
		Bucket: oss.Ptr(bucketName),
		Key:    oss.Ptr(objectName),
	},
		oss.PresignExpires(expireTime),
	)
	if err != nil {
		return "", err
	}
	return result.URL, nil
}

func callBailian(imgURL string, modelName string) (string, error) {
	apiKey := os.Getenv("DASHSCOPE_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("DASHSCOPE_API_KEY not configured")
	}

	client := &http.Client{Timeout: 90 * time.Second}
	requestBody := struct {
		Model    string `json:"model"`
		Messages []struct {
			Role    string `json:"role"`
			Content []struct {
				Type     string `json:"type"`
				Text     string `json:"text,omitempty"`
				ImageURL *struct {
					URL string `json:"url"`
				} `json:"image_url,omitempty"`
			} `json:"content"`
		} `json:"messages"`
		Stream        bool `json:"stream"`
		StreamOptions *struct {
			IncludeUsage bool `json:"include_usage"`
		} `json:"stream_options,omitempty"`
		EnableThinking bool `json:"enable_thinking,omitempty"`
	}{
		Model: modelName,
		Messages: []struct {
			Role    string `json:"role"`
			Content []struct {
				Type     string `json:"type"`
				Text     string `json:"text,omitempty"`
				ImageURL *struct {
					URL string `json:"url"`
				} `json:"image_url,omitempty"`
			} `json:"content"`
		}{
			{
				Role: "user",
				Content: []struct {
					Type     string `json:"type"`
					Text     string `json:"text,omitempty"`
					ImageURL *struct {
						URL string `json:"url"`
					} `json:"image_url,omitempty"`
				}{
					{
						Type: "image_url",
						ImageURL: &struct {
							URL string `json:"url"`
						}{URL: imgURL},
					},
					{
						Type: "text",
						Text: "你现在是一个微生物领域的专家。请你根据菌落的形状、边缘、表面质地、颜色等维度，判断这张图片中是否有杂菌。在回复的开始按\u201c污染可能性：低/中/高\\n\u201d回复，并陈述你的理由。",
					},
				},
			},
		},
		Stream: true,
		StreamOptions: &struct {
			IncludeUsage bool `json:"include_usage"`
		}{IncludeUsage: true},
		EnableThinking: false,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	content := ""
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
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
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		json.Unmarshal([]byte(data), &chunk)

		for _, c := range chunk.Choices {
			if c.Delta.Content != "" {
				content += c.Delta.Content
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read stream: %w", err)
	}

	return content, nil
}

func AnalyzeColony(uuid string, plateid int, timestamp string) AnalyzeResponse {
	modelName := os.Getenv("MODEL_NAME")
	if modelName == "" {
		return AnalyzeResponse{Success: false, Message: "MODEL_NAME not configured"}
	}

	loc, _ := time.LoadLocation("Asia/Shanghai")
	if loc == nil {
		loc = time.UTC
	}
	ts, err := time.ParseInLocation("20060102-150405", timestamp, loc)
	if err != nil {
		return AnalyzeResponse{Success: false, Message: "Invalid timestamp format: " + err.Error()}
	}

	truncatedUs := ts.UnixMicro() / 1e6 * 1e6

	client := getTimeseriesClient()
	tableName := os.Getenv("COLONY_TABLE_NAME")
	measurementName := os.Getenv("COLONY_MEASURE_NAME")

	timeseriesKey := tablestore.NewTimeseriesKey()
	timeseriesKey.SetMeasurementName(measurementName)
	timeseriesKey.SetDataSource(uuid)
	timeseriesKey.AddTag("plate_id", strconv.Itoa(plateid))

	getReq := tablestore.NewGetTimeseriesDataRequest(tableName)
	getReq.SetTimeseriesKey(timeseriesKey)
	getReq.SetTimeRange(truncatedUs, truncatedUs+1)
	getReq.SetLimit(-1)

	getResp, err := client.GetTimeseriesData(getReq)
	if err != nil {
		return AnalyzeResponse{Success: false, Message: "Failed to read table: " + err.Error()}
	}

	found := false
	var existingFields map[string]*tablestore.ColumnValue
	for i := 0; i < len(getResp.GetRows()); i++ {
		if getResp.GetRows()[i].GetTimeInus() == truncatedUs {
			existingFields = getResp.GetRows()[i].GetFieldsMap()
			found = true
			break
		}
	}

	if !found {
		return AnalyzeResponse{Success: false, Message: "No matching colony record found"}
	}

	imgPath := uuid + "/" +
		strconv.Itoa(plateid) + "/" +
		timestamp + ".bmp"

	imgURL, err := signDownloadURL(imgPath, 10*time.Minute)
	if err != nil {
		return AnalyzeResponse{Success: false, Message: "Failed to sign image URL: " + err.Error()}
	}

	reply, err := callBailian(imgURL, modelName)
	if err != nil {
		return AnalyzeResponse{Success: false, Message: "AI inference failed: " + err.Error()}
	}

	writeKey := tablestore.NewTimeseriesKey()
	writeKey.SetMeasurementName(measurementName)
	writeKey.SetDataSource(uuid)
	writeKey.AddTag("plate_id", strconv.Itoa(plateid))

	writeRow := tablestore.NewTimeseriesRow(writeKey)
	writeRow.SetTimeInus(truncatedUs)

	for key, value := range existingFields {
		if key == "reply" {
			continue
		}
		writeRow.AddField(key, value)
	}
	writeRow.AddField("reply",
		tablestore.NewColumnValue(tablestore.ColumnType_STRING, reply))

	putReq := tablestore.NewPutTimeseriesDataRequest(tableName)
	putReq.AddTimeseriesRows(writeRow)

	_, err = client.PutTimeseriesData(putReq)
	if err != nil {
		return AnalyzeResponse{Success: false, Message: "Failed to write reply: " + err.Error()}
	}

	return AnalyzeResponse{Success: true, Reply: reply}
}
