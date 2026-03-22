package server

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"CursorAgent/config"
	"CursorAgent/util"
)

type PluginListener struct {
	port int
	opt  *config.Option
	srv  *http.Server
}

func NewPluginListener(port int, opt *config.Option) *PluginListener {
	if port <= 0 {
		port = 9150
	}
	l := &PluginListener{port: port, opt: opt}
	mux := http.NewServeMux()
	// 插件请求路径示例：
	// - GET  /api/review/checked/<task>?date=YYYY-MM-DD
	// - POST /api/review/done/<task>?date=YYYY-MM-DD
	mux.HandleFunc("/api/review/checked/", l.handleChecked)
	mux.HandleFunc("/api/review/done/", l.handleDone)
	l.srv = &http.Server{
		Addr:    "localhost:" + strconv.Itoa(l.port),
		Handler: mux,
	}
	return l
}

func (l *PluginListener) addr() string {
	return "localhost:" + strconv.Itoa(l.port)
}

func (l *PluginListener) findLaunchTask(taskName string) *config.Task {
	if l.opt == nil {
		return nil
	}
	for i := range l.opt.Tasks {
		t := &l.opt.Tasks[i]
		if t.Name == taskName {
			return t
		}
	}
	return nil
}

func (l *PluginListener) handleChecked(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	date := r.URL.Query().Get("date")
	if date == "" {
		http.Error(w, "missing date", http.StatusBadRequest)
		return
	}
	// 从路径解析：/api/review/checked/<task>
	taskName := ""
	const prefix = "/api/review/checked/"
	if strings.HasPrefix(r.URL.Path, prefix) {
		taskName = strings.TrimPrefix(r.URL.Path, prefix)
		taskName = strings.Trim(taskName, "/")
	}
	if taskName == "" {
		http.Error(w, "missing task", http.StatusBadRequest)
		return
	}
	task := l.findLaunchTask(taskName)
	if task == nil {
		http.Error(w, "task not registered", http.StatusNotFound)
		return
	}
	resultPath := task.Output.ResultFilePath(date)
	_, err := os.Stat(resultPath)
	checked := err == nil
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"checked": checked})
	log.Printf("plugin_listener: handleChecked: date: %s，taskName: %s， checked: %t", date, taskName, checked)
}

func (l *PluginListener) handleDone(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	date := r.URL.Query().Get("date")
	if date == "" {
		http.Error(w, "missing date", http.StatusBadRequest)
		return
	}
	// 从路径解析：/api/review/done/<task>
	taskName := ""
	const prefix = "/api/review/done/"
	if strings.HasPrefix(r.URL.Path, prefix) {
		taskName = strings.TrimPrefix(r.URL.Path, prefix)
		taskName = strings.Trim(taskName, "/")
	}
	if taskName == "" {
		http.Error(w, "missing task", http.StatusBadRequest)
		return
	}
	task := l.findLaunchTask(taskName)
	if task == nil {
		http.Error(w, "can not find task", http.StatusNotFound)
		return
	}
	resultPath := task.Output.ResultFilePath(date)
	uploaded := false
	if task.Output.UploadURL != "" {
		if _, err := os.Stat(resultPath); err == nil {
			if err := util.UploadFile(task.Output.UploadURL, resultPath); err != nil {
				log.Printf("plugin_listener: upload failed: %v", err)
			} else {
				uploaded = true
			}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"uploaded": uploaded})
	log.Printf("plugin_listener: handleDone: date: %s，taskName: %s， uploaded: %t", date, taskName, uploaded)
}

func (l *PluginListener) Start() {
	go func() {
		log.Printf("plugin_listener: listening on %s", l.addr())
		if err := l.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("plugin_listener: %v", err)
		}
	}()
}

func (l *PluginListener) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return l.srv.Shutdown(ctx)
}
