package web

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/aliyun/aliyun-tablestore-go-sdk/tablestore"
)

type CorrectionResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

func SaveColonyCorrection(uuid string, plateid int, timestamp string, userBoxes json.RawMessage) CorrectionResponse {
	if len(userBoxes) == 0 || !json.Valid(userBoxes) {
		return CorrectionResponse{Success: false, Message: "Invalid user_boxes JSON"}
	}

	loc, _ := time.LoadLocation("Asia/Shanghai")
	if loc == nil {
		loc = time.UTC
	}
	ts, err := time.ParseInLocation("20060102-150405", timestamp, loc)
	if err != nil {
		return CorrectionResponse{Success: false, Message: "Invalid timestamp format: " + err.Error()}
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
		return CorrectionResponse{Success: false, Message: "Failed to read table: " + err.Error()}
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
		return CorrectionResponse{Success: false, Message: "No matching colony record found"}
	}

	writeRow := tablestore.NewTimeseriesRow(timeseriesKey)
	writeRow.SetTimeInus(truncatedUs)
	for key, value := range existingFields {
		if key == "user_boxes" {
			continue
		}
		writeRow.AddField(key, value)
	}
	writeRow.AddField("user_boxes",
		tablestore.NewColumnValue(tablestore.ColumnType_STRING, string(userBoxes)))

	putReq := tablestore.NewPutTimeseriesDataRequest(tableName)
	putReq.AddTimeseriesRows(writeRow)
	putResp, err := client.PutTimeseriesData(putReq)
	if err != nil {
		return CorrectionResponse{Success: false, Message: fmt.Sprintf("Failed to write user correction: %v", err)}
	}
	if putResp == nil {
		return CorrectionResponse{Success: false, Message: "Failed to write user correction: empty response"}
	}
	failedRows := putResp.GetFailedRowResults()
	if len(failedRows) > 0 {
		failed := failedRows[0]
		return CorrectionResponse{Success: false, Message: fmt.Sprintf("Failed to write user correction row %d: %v", failed.Index, failed.Error)}
	}

	return CorrectionResponse{Success: true}
}
