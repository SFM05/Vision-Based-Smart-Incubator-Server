package main

import (
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"mqtt_listener/web"

	"github.com/joho/godotenv"
)

func recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				slog.Error(fmt.Sprintf("Panic recovered: %v", err))
				http.Error(w, `{"sucess":false,"message":"internal server error"}`, http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found, using environment variables:", err)
	}

	fs := http.FileServer(http.Dir("static"))

	mux := http.NewServeMux()
	mux.HandleFunc("/api/devices", handleDevicesQuery)
	mux.HandleFunc("/api/env", handleEnvQuery)
	mux.HandleFunc("/api/colony", handleColonyQuery)
	mux.HandleFunc("/api/colony/analyze", handleColonyAnalyze)
	mux.HandleFunc("/api/colony/correction", handleColonyCorrection)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/env.html", http.StatusMovedPermanently)
			return
		}
		fs.ServeHTTP(w, r)
	})

	handler := recoverMiddleware(mux)

	log.Println("web server starting on :8080")
	server := &http.Server{
		Addr:              ":8080",
		Handler:           handler,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      120 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Web server failed: %v", err)
	}
}

func handleDevicesQuery(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	defer func() { log.Printf("%s completed in %s", r.URL.Path, time.Since(started)) }()

	if r.Method != http.MethodGet {
		http.Error(w, `{"success":false,"message":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	result := web.GetDevices()
	if !result.Success {
		w.WriteHeader(http.StatusInternalServerError)
	}
	json.NewEncoder(w).Encode(result)
}

func handleEnvQuery(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	defer func() { log.Printf("%s completed in %s", r.URL.Path, time.Since(started)) }()

	if r.Method != http.MethodGet {
		http.Error(w, `{"sucess":false,"message":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	uuid := r.URL.Query().Get("uuid")
	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")

	if uuid == "" || startStr == "" || endStr == "" {
		http.Error(w, `{"sucess":false,"message":"missing required params: uuid, start, end"}`, http.StatusBadRequest)
		return
	}

	startMicro, err := strconv.ParseInt(startStr, 10, 64)
	if err != nil {
		http.Error(w, `{"sucess":false,"message":"invalid start param"}`, http.StatusBadRequest)
		return
	}

	endMicro, err := strconv.ParseInt(endStr, 10, 64)
	if err != nil {
		http.Error(w, `{"sucess":false,"message":"invalid end param"}`, http.StatusBadRequest)
		return
	}

	if startMicro >= endMicro {
		http.Error(w, `{"sucess":false,"message":"start must be before end"}`, http.StatusBadRequest)
		return
	}

	if endMicro-startMicro > 7*24*3600*1000000 {
		http.Error(w, `{"sucess":false,"message":"time range exceeds 7 days"}`, http.StatusBadRequest)
		return
	}

	start := time.UnixMicro(startMicro)
	end := time.UnixMicro(endMicro)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	result := web.GetEnv(uuid, start, end)
	if strings.Contains(result, `"sucess":false`) {
		w.WriteHeader(http.StatusInternalServerError)
	}
	w.Write([]byte(result))
}

func handleColonyQuery(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	defer func() { log.Printf("%s completed in %s", r.URL.Path, time.Since(started)) }()

	if r.Method != http.MethodGet {
		http.Error(w, `{"success":false,"message":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	uuid := r.URL.Query().Get("uuid")
	plateidStr := r.URL.Query().Get("plateid")
	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")

	if uuid == "" || plateidStr == "" || startStr == "" || endStr == "" {
		http.Error(w, `{"success":false,"message":"missing required params: uuid, plateid, start, end"}`, http.StatusBadRequest)
		return
	}

	plateid, err := strconv.Atoi(plateidStr)
	if err != nil || plateid < 0 {
		http.Error(w, `{"success":false,"message":"invalid plateid param"}`, http.StatusBadRequest)
		return
	}

	startMicro, err := strconv.ParseInt(startStr, 10, 64)
	if err != nil {
		http.Error(w, `{"success":false,"message":"invalid start param"}`, http.StatusBadRequest)
		return
	}

	endMicro, err := strconv.ParseInt(endStr, 10, 64)
	if err != nil {
		http.Error(w, `{"success":false,"message":"invalid end param"}`, http.StatusBadRequest)
		return
	}

	if startMicro >= endMicro {
		http.Error(w, `{"success":false,"message":"start must be before end"}`, http.StatusBadRequest)
		return
	}

	if endMicro-startMicro > 7*24*3600*1000000 {
		http.Error(w, `{"success":false,"message":"time range exceeds 7 days"}`, http.StatusBadRequest)
		return
	}

	start := time.UnixMicro(startMicro)
	end := time.UnixMicro(endMicro)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	result := web.GetColony(uuid, plateid, start, end)
	if strings.Contains(result, `"success":false`) {
		w.WriteHeader(http.StatusInternalServerError)
	}
	w.Write([]byte(result))
}

func handleColonyAnalyze(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"success":false,"message":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		UUID      string `json:"uuid"`
		PlateID   int    `json:"plateid"`
		Timestamp string `json:"timestamp"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"success":false,"message":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.UUID == "" || req.PlateID < 0 || req.Timestamp == "" {
		http.Error(w, `{"success":false,"message":"missing required params: uuid, plateid, timestamp"}`, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	result := web.AnalyzeColony(req.UUID, req.PlateID, req.Timestamp)
	json.NewEncoder(w).Encode(result)
}

func handleColonyCorrection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"success":false,"message":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		UUID      string          `json:"uuid"`
		PlateID   int             `json:"plateid"`
		Timestamp string          `json:"timestamp"`
		UserBoxes json.RawMessage `json:"user_boxes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"success":false,"message":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.UUID == "" || req.PlateID < 0 || req.Timestamp == "" || len(req.UserBoxes) == 0 {
		http.Error(w, `{"success":false,"message":"missing required params: uuid, plateid, timestamp, user_boxes"}`, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	result := web.SaveColonyCorrection(req.UUID, req.PlateID, req.Timestamp, req.UserBoxes)
	if !result.Success {
		w.WriteHeader(http.StatusInternalServerError)
	}
	json.NewEncoder(w).Encode(result)
}
