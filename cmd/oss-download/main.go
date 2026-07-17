package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		log.Fatal(err)
	}

	bucket := os.Getenv("BUCKET_NAME")
	region := os.Getenv("REGION")
	if bucket == "" || region == "" {
		log.Fatal("BUCKET_NAME and REGION must be set")
	}

	destination := "oss-download"
	if len(os.Args) > 2 {
		log.Fatalf("usage: %s [destination]", os.Args[0])
	}
	if len(os.Args) == 2 {
		destination = os.Args[1]
	}
	absDestination, err := filepath.Abs(destination)
	if err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(absDestination, 0755); err != nil {
		log.Fatal(err)
	}

	client := oss.NewClient(oss.LoadDefaultConfig().
		WithCredentialsProvider(credentials.NewEnvironmentVariableCredentialsProvider()).
		WithRegion(region))
	paginator := client.NewListObjectsV2Paginator(&oss.ListObjectsV2Request{
		Bucket:  oss.Ptr(bucket),
		MaxKeys: 1000,
	})

	ctx := context.Background()
	var count int
	var totalBytes int64
	for paginator.HasNext() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			log.Fatal(err)
		}
		for _, object := range page.Contents {
			if object.Key == nil || *object.Key == "" || strings.HasSuffix(*object.Key, "/") {
				continue
			}
			path, err := localPath(absDestination, *object.Key)
			if err != nil {
				log.Fatalf("unsafe object key %q: %v", *object.Key, err)
			}
			if err := download(ctx, client, bucket, *object.Key, path); err != nil {
				log.Fatalf("download %q: %v", *object.Key, err)
			}
			count++
			totalBytes += object.Size
			log.Printf("[%d] %s", count, *object.Key)
		}
	}

	log.Printf("Downloaded %d objects (%d bytes) to %s", count, totalBytes, absDestination)
}

func localPath(destination, key string) (string, error) {
	path := filepath.Join(destination, filepath.FromSlash(key))
	relative, err := filepath.Rel(destination, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", fmt.Errorf("path escapes destination")
	}
	return path, nil
}

func download(ctx context.Context, client *oss.Client, bucket, key, path string) error {
	result, err := client.GetObject(ctx, &oss.GetObjectRequest{
		Bucket: oss.Ptr(bucket),
		Key:    oss.Ptr(key),
	})
	if err != nil {
		return err
	}
	defer result.Body.Close()

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	temporaryPath := path + ".partial"
	file, err := os.Create(temporaryPath)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(file, result.Body)
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(temporaryPath, path)
}
