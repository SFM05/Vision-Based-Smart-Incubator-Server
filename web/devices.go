package web

import (
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/aliyun/aliyun-tablestore-go-sdk/tablestore"
)

type DeviceMetaData struct {
	UUID   string `json:"uuid"`
	Plates []int  `json:"plates"`
}

type DevicesResponse struct {
	Success bool             `json:"success"`
	Message string           `json:"message,omitempty"`
	Devices []DeviceMetaData `json:"devices,omitempty"`
}

var devicesCache = struct {
	sync.Mutex
	response  DevicesResponse
	expiresAt time.Time
}{}

const devicesCacheTTL = 60 * time.Second

func queryTimeseriesMetas(client *tablestore.TimeseriesClient, tableName string, measurementName string) ([]*tablestore.TimeseriesMeta, error) {
	if tableName == "" || measurementName == "" {
		return nil, fmt.Errorf("missing timeseries table or measurement config")
	}

	request := tablestore.NewQueryTimeseriesMetaRequest(tableName)
	request.SetCondition(tablestore.NewMeasurementQueryCondition(tablestore.OP_EQUAL, measurementName))
	request.SetLimit(-1)

	metas := []*tablestore.TimeseriesMeta{}
	for {
		response, err := client.QueryTimeseriesMeta(request)
		if err != nil {
			return nil, err
		}
		metas = append(metas, response.GetTimeseriesMetas()...)

		nextToken := response.GetNextToken()
		if len(nextToken) == 0 {
			break
		}
		request.SetNextToken(nextToken)
	}

	return metas, nil
}

// GetDevices 获取已写入时序表的设备 UUID 和菌落盘位号。
func GetDevices() DevicesResponse {
	devicesCache.Lock()
	if time.Now().Before(devicesCache.expiresAt) {
		cached := devicesCache.response
		devicesCache.Unlock()
		return cached
	}
	devicesCache.Unlock()

	client := getTimeseriesClient()

	devicePlates := map[string]map[int]bool{}
	ensureDevice := func(uuid string) {
		if uuid == "" {
			return
		}
		if _, ok := devicePlates[uuid]; !ok {
			devicePlates[uuid] = map[int]bool{}
		}
	}

	envMetas, err := queryTimeseriesMetas(client, os.Getenv("ENV_TABLE_NAME"), os.Getenv("ENV_MEASURE_NAME"))
	if err != nil {
		slog.Error(fmt.Sprintf("Query env timeseries meta failed: %v", err))
		return DevicesResponse{Success: false, Message: err.Error()}
	}
	for _, meta := range envMetas {
		if meta == nil || meta.GetTimeseriesKey() == nil {
			continue
		}
		ensureDevice(meta.GetTimeseriesKey().GetDataSource())
	}

	colonyMetas, err := queryTimeseriesMetas(client, os.Getenv("COLONY_TABLE_NAME"), os.Getenv("COLONY_MEASURE_NAME"))
	if err != nil {
		slog.Error(fmt.Sprintf("Query colony timeseries meta failed: %v", err))
		return DevicesResponse{Success: false, Message: err.Error()}
	}
	for _, meta := range colonyMetas {
		if meta == nil || meta.GetTimeseriesKey() == nil {
			continue
		}
		key := meta.GetTimeseriesKey()
		uuid := key.GetDataSource()
		ensureDevice(uuid)

		plateID, ok := key.GetTags()["plate_id"]
		if !ok {
			continue
		}
		plate, err := strconv.Atoi(plateID)
		if err != nil || plate < 0 {
			slog.Warn(fmt.Sprintf("Skipping invalid plate_id %q for uuid %q", plateID, uuid))
			continue
		}
		devicePlates[uuid][plate] = true
	}

	uuids := make([]string, 0, len(devicePlates))
	for uuid := range devicePlates {
		uuids = append(uuids, uuid)
	}
	sort.Strings(uuids)

	response := DevicesResponse{Success: true}
	for _, uuid := range uuids {
		plates := make([]int, 0, len(devicePlates[uuid]))
		for plate := range devicePlates[uuid] {
			plates = append(plates, plate)
		}
		sort.Ints(plates)
		response.Devices = append(response.Devices, DeviceMetaData{UUID: uuid, Plates: plates})
	}

	devicesCache.Lock()
	devicesCache.response = response
	devicesCache.expiresAt = time.Now().Add(devicesCacheTTL)
	devicesCache.Unlock()

	return response
}
