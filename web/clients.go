package web

import (
	"os"
	"sync"

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"
	"github.com/aliyun/aliyun-tablestore-go-sdk/tablestore"
)

var (
	tablestoreClientOnce sync.Once
	tablestoreClient     *tablestore.TimeseriesClient

	ossClientOnce sync.Once
	ossClient     *oss.Client
)

func getTimeseriesClient() *tablestore.TimeseriesClient {
	tablestoreClientOnce.Do(func() {
		tablestoreClient = tablestore.NewTimeseriesClient(
			os.Getenv("TABLE_ENDPOINT"),
			os.Getenv("TABLE_INSTANCE_NAME"),
			os.Getenv("TABLESTORE_ACCESS_KEY_ID"),
			os.Getenv("TABLESTORE_ACCESS_KEY_SECRET"),
		)
	})
	return tablestoreClient
}

func getOSSClient() *oss.Client {
	ossClientOnce.Do(func() {
		cfg := oss.LoadDefaultConfig().
			WithCredentialsProvider(credentials.NewEnvironmentVariableCredentialsProvider()).
			WithRegion(os.Getenv("REGION"))
		ossClient = oss.NewClient(cfg)
	})
	return ossClient
}
