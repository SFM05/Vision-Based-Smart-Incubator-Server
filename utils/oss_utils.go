// OSS工具函数
package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"

	MQTT "github.com/eclipse/paho.mqtt.golang"
)

var (
	ErrRegionRequired     = errors.New("region is required")
	ErrBucketRequired     = errors.New("bucket name is required")
	ErrObjectNameRequired = errors.New("object name is required")
)

// 文件是否存在
func Existed(object_name string) bool {

	region := os.Getenv("REGION")
	bucket_name := os.Getenv("BUCKET_NAME")

	// 加载默认配置并设置凭证提供者和区域
	cfg := oss.LoadDefaultConfig().
		WithCredentialsProvider(credentials.NewEnvironmentVariableCredentialsProvider()).
		WithRegion(region)

	// 创建OSS客户端
	client := oss.NewClient(cfg)

	existed, err := client.IsObjectExist(context.TODO(), bucket_name, object_name)
	if err != nil {
		return false
	} else {
		if existed {
			return true
		} else {
			return false
		}
	}
}

// SignURL 生成预签名上传URL
func signUploadURL(object_name string, expire_time time.Duration) (string, error) {
	region := os.Getenv("REGION")
	bucket_name := os.Getenv("BUCKET_NAME")

	// 加载默认配置并设置凭证提供者和区域
	cfg := oss.LoadDefaultConfig().
		WithCredentialsProvider(credentials.NewEnvironmentVariableCredentialsProvider()).
		WithRegion(region)

	// 创建OSS客户端
	client := oss.NewClient(cfg)

	// 生成PutObject的预签名URL
	result, err := client.Presign(context.TODO(), &oss.PutObjectRequest{
		Bucket: oss.Ptr(bucket_name),
		Key:    oss.Ptr(object_name),
	},
		oss.PresignExpires(expire_time),
	)
	if err != nil {
		return "", err
	}
	return result.URL, nil
}

// signDownloadURL 生成预签名下载url
func signDownloadURL(object_name string, expire_time time.Duration) (string, error) {
	region := os.Getenv("REGION")
	bucket_name := os.Getenv("BUCKET_NAME")

	// 加载默认配置并设置凭证提供者和区域
	cfg := oss.LoadDefaultConfig().
		WithCredentialsProvider(credentials.NewEnvironmentVariableCredentialsProvider()).
		WithRegion(region)

	// 创建OSS客户端
	client := oss.NewClient(cfg)

	// 生成GetObject的预签名URL
	result, err := client.Presign(context.TODO(), &oss.GetObjectRequest{
		Bucket: oss.Ptr(bucket_name),
		Key:    oss.Ptr(object_name),
	},
		oss.PresignExpires(expire_time),
	)
	if err != nil {
		return "", err
	}
	return result.URL, nil
}

// uploadMessage MQTT发送URL信息
func uploadMessage(client MQTT.Client,
	uuid string,
	timestamp time.Time,
	status bool,
	path string,
	url string) {
	topic := "server" + "/" + uuid + "/" + "upload"
	qos := 1
	retained := false

	msg := struct {
		Timestamp string `json:"timestamp"`
		Success   bool   `json:"success"`
		Path      string `json:"path"`
		ImgURL    string `json:"url"`
	}{timestamp.Format("2006-01-02 15:04:05"), status, path, url}

	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(false) // 禁用转义
	encoder.Encode(msg)

	token := client.Publish(topic, byte(qos), retained, buffer.Bytes())
	token.Wait()
}

// Upload 回调函数，处理来自mcu的上传请求
func OnUploadRequest(client MQTT.Client, uuid string, payload string) {
	// {"timestamp":string, "plateid":int, "imgpath":string, "txtpath":string, "number":int}
	var json_result struct {
		Timestamp string `json:"timestamp"`
		PlateID   int    `json:"plateid"`
		ImgPath   string `json:"imgpath"`
		TxtPath   string `json:"txtpath"`
		Number    int    `json:"number"`
	}
	err := json.Unmarshal([]byte(payload), &json_result)
	if err != nil {
		slog.Error(fmt.Sprintf("Encounter error when decoding json: %v", err))
		slog.Error(fmt.Sprintf("    Original message: %s", payload))
		return
	}

	// 解析时间
	loc, _ := time.LoadLocation("Asia/Shanghai")
	timestamp, err := time.ParseInLocation("20060102-150405", json_result.Timestamp, loc)
	if err != nil {
		// 解析时间失败时使用服务器时间作为替代
		slog.Warn(fmt.Sprintf("Time parse fail: %v", err))
		slog.Warn(fmt.Sprintf("    Original time: %s", json_result.Timestamp))
		slog.Warn("Using server time instead")
		timestamp = time.Now().In(loc)
	}

	// 生成图片的预签名URL
	img_path := uuid + "/" +
		strconv.Itoa(json_result.PlateID) + "/" +
		timestamp.Format("20060102-150405") + ".bmp"
	img_url, err := signUploadURL(img_path, 10*time.Minute)
	if err != nil {
		slog.Error(fmt.Sprintf("Sign URL failed: %v", err))
		return
	}
	uploadMessage(client, uuid, timestamp, true, json_result.ImgPath, img_url)

	// 生成文本记录的预签名URL
	txt_path := uuid + "/" +
		strconv.Itoa(json_result.PlateID) + "/" +
		timestamp.Format("20060102-150405") + ".txt"
	txt_url, err := signUploadURL(txt_path, 10*time.Minute)
	if err != nil {
		slog.Error(fmt.Sprintf("Sign URL failed: %v", err))
		return
	}
	uploadMessage(client, uuid, timestamp, true, json_result.TxtPath, txt_url)

	// 上报文件路径和数量
	RecordColonyData(uuid,
		json_result.PlateID,
		timestamp,
		img_path,
		txt_path,
		json_result.Number)

	slog.Info("Publish upload reply success")

	time.Sleep(60 * time.Second)
	if Existed(img_path) {
		slog.Info(fmt.Sprintf("File upload secess after 60s: %s", img_path))
		// UploadSucess(uuid, timestamp, json_result.PlateID)
		return
	}
	time.Sleep(60 * time.Second)
	if Existed(img_path) {
		slog.Info(fmt.Sprintf("File upload secess after 120s: %s", img_path))
		// UploadSucess(uuid, timestamp, json_result.PlateID)
		return
	}
	time.Sleep(480 * time.Second)
	if Existed(img_path) {
		slog.Info(fmt.Sprintf("File upload secess after 600s: %s", img_path))
		// UploadSucess(uuid, timestamp, json_result.PlateID)
		return
	}
	slog.Info(fmt.Sprintf("Fail to receive file: %s", img_path))
}
