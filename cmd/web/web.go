package main

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"mqtt_listener/web"
)

func main() {
	fs := http.FileServer(http.Dir("static"))

	http.HandleFunc("/api/env", handleEnvQuery)
	http.HandleFunc("/api/colony", handleColonyQuery)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/env.html", http.StatusMovedPermanently)
			return
		}
		fs.ServeHTTP(w, r)
	})

	log.Println("web server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func handleEnvQuery(w http.ResponseWriter, r *http.Request) {
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

	start := time.UnixMicro(startMicro)
	end := time.UnixMicro(endMicro)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write([]byte(web.GetEnv(uuid, start, end)))
}

func handleColonyQuery(w http.ResponseWriter, r *http.Request) {
	uuid := r.URL.Query().Get("uuid")
	plateidStr := r.URL.Query().Get("plateid")
	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")

	if uuid == "" || plateidStr == "" || startStr == "" || endStr == "" {
		http.Error(w, `{"success":false,"message":"missing required params: uuid, plateid, start, end"}`, http.StatusBadRequest)
		return
	}

	plateid, err := strconv.Atoi(plateidStr)
	if err != nil {
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

	start := time.UnixMicro(startMicro)
	end := time.UnixMicro(endMicro)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write([]byte(web.GetColony(uuid, plateid, start, end)))
}
