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

// Existed 检查文件是否存在，返回 (exists, error)
func Existed(object_name string) (bool, error) {
	region := os.Getenv("REGION")
	bucket_name := os.Getenv("BUCKET_NAME")

	cfg := oss.LoadDefaultConfig().
		WithCredentialsProvider(credentials.NewEnvironmentVariableCredentialsProvider()).
		WithRegion(region)

	client := oss.NewClient(cfg)

	existed, err := client.IsObjectExist(context.TODO(), bucket_name, object_name)
	if err != nil {
		return false, err
	}
	return existed, nil
}

// signUploadURL 生成预签名上传URL
func signUploadURL(object_name string, expire_time time.Duration) (string, error) {
	region := os.Getenv("REGION")
	bucket_name := os.Getenv("BUCKET_NAME")

	cfg := oss.LoadDefaultConfig().
		WithCredentialsProvider(credentials.NewEnvironmentVariableCredentialsProvider()).
		WithRegion(region)

	client := oss.NewClient(cfg)

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

	cfg := oss.LoadDefaultConfig().
		WithCredentialsProvider(credentials.NewEnvironmentVariableCredentialsProvider()).
		WithRegion(region)

	client := oss.NewClient(cfg)

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
	}{timestamp.Format("20060102-150405"), status, path, url}

	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(false) // 禁用转义
	encoder.Encode(msg)

	token := client.Publish(topic, byte(qos), retained, buffer.Bytes())
	token.Wait()
	if token.Error() != nil {
		slog.Error(fmt.Sprintf("Failed to publish upload reply: %v", token.Error()))
	}
}

// OnUploadRequest 回调函数，处理来自mcu的上传请求
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

	loc := loadLocation()
	timestamp, err := time.ParseInLocation("20060102-150405", json_result.Timestamp, loc)
	if err != nil {
		slog.Warn(fmt.Sprintf("Time parse fail: %v", err))
		slog.Warn(fmt.Sprintf("    Original time: %s", json_result.Timestamp))
		slog.Warn("Using server time instead")
		timestamp = time.Now().In(loc)
	}

	img_path := uuid + "/" +
		strconv.Itoa(json_result.PlateID) + "/" +
		timestamp.Format("20060102-150405") + ".bmp"
	txt_path := uuid + "/" +
		strconv.Itoa(json_result.PlateID) + "/" +
		timestamp.Format("20060102-150405") + ".txt"

	// Generate both presign URLs before publishing either
	img_url, err := signUploadURL(img_path, 10*time.Minute)
	if err != nil {
		slog.Error(fmt.Sprintf("Sign image URL failed: %v", err))
		uploadMessage(client, uuid, timestamp, false, json_result.ImgPath, "")
		return
	}
	txt_url, err := signUploadURL(txt_path, 10*time.Minute)
	if err != nil {
		slog.Error(fmt.Sprintf("Sign text URL failed: %v", err))
		uploadMessage(client, uuid, timestamp, false, json_result.TxtPath, "")
		return
	}

	// Both URLs signed successfully, publish both replies
	uploadMessage(client, uuid, timestamp, true, json_result.ImgPath, img_url)
	uploadMessage(client, uuid, timestamp, true, json_result.TxtPath, txt_url)

	slog.Info("Publish upload reply success")

	// Do not create a queryable record until both uploaded objects are present.
	recordColonyData := func() {
		RecordColonyData(uuid,
			json_result.PlateID,
			timestamp,
			img_path,
			txt_path,
			json_result.Number)
	}

	// Poll OSS for both files with proper error handling
	imgExists := false
	txtExists := false

	time.Sleep(60 * time.Second)
	imgExists, imgErr := Existed(img_path)
	if imgErr != nil {
		slog.Warn(fmt.Sprintf("Error checking image after 60s: %v", imgErr))
	} else if imgExists {
		slog.Info(fmt.Sprintf("File upload success after 60s: %s", img_path))
	}
	txtExists, txtErr := Existed(txt_path)
	if txtErr != nil {
		slog.Warn(fmt.Sprintf("Error checking text after 60s: %v", txtErr))
	} else if txtExists {
		slog.Info(fmt.Sprintf("File upload success after 60s: %s", txt_path))
	}
	if imgExists && txtExists {
		recordColonyData()
		return
	}

	time.Sleep(60 * time.Second)
	if !imgExists {
		imgExists, imgErr = Existed(img_path)
		if imgErr != nil {
			slog.Warn(fmt.Sprintf("Error checking image after 120s: %v", imgErr))
		} else if imgExists {
			slog.Info(fmt.Sprintf("File upload success after 120s: %s", img_path))
		}
	}
	if !txtExists {
		txtExists, txtErr = Existed(txt_path)
		if txtErr != nil {
			slog.Warn(fmt.Sprintf("Error checking text after 120s: %v", txtErr))
		} else if txtExists {
			slog.Info(fmt.Sprintf("File upload success after 120s: %s", txt_path))
		}
	}
	if imgExists && txtExists {
		recordColonyData()
		return
	}

	time.Sleep(480 * time.Second)
	if !imgExists {
		imgExists, imgErr = Existed(img_path)
		if imgErr != nil {
			slog.Warn(fmt.Sprintf("Error checking image after 600s: %v", imgErr))
		} else if imgExists {
			slog.Info(fmt.Sprintf("File upload success after 600s: %s", img_path))
		}
	}
	if !txtExists {
		txtExists, txtErr = Existed(txt_path)
		if txtErr != nil {
			slog.Warn(fmt.Sprintf("Error checking text after 600s: %v", txtErr))
		} else if txtExists {
			slog.Info(fmt.Sprintf("File upload success after 600s: %s", txt_path))
		}
	}
	if imgExists && txtExists {
		recordColonyData()
		return
	}

	slog.Info(fmt.Sprintf("Fail to receive files: img=%s (exists=%v), txt=%s (exists=%v)", img_path, imgExists, txt_path, txtExists))
}
