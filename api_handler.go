package main

import (
	"encoding/json"
	"fmt"
	"github.com/rschlaikjer/go-apache-logformat"
	"log"
	"net/http"
	"os"
)

const apache_log_format = `%h %l %u %t "%r" %>s %b "%{Referer}i" "%{User-agent}i"`

type ApiError struct {
	Success int
	Error   string
}

type ApiHandler struct {
	Servlets  map[string]func(http.ResponseWriter, *http.Request)
	AccessLog *apachelog.ApacheLog
}

func NewApiHandler(server_config *Config) *ApiHandler {
	h := new(ApiHandler)
	h.SetAccessLog(server_config)
	h.Servlets = make(map[string]func(http.ResponseWriter, *http.Request))
	return h
}

func (t *ApiHandler) SetAccessLog(server_config *Config) {
	if !server_config.Arguments.LogToStderr {
		if _, err := os.Stat(server_config.Logging.AccessLogFile); os.IsNotExist(err) {
			log_file, err := os.Create(server_config.Logging.AccessLogFile)
			if err != nil {
				log.Fatal("Log: Create: ", err.Error())
			}
			t.AccessLog = apachelog.NewApacheLog(log_file, apache_log_format)
		} else {
			log_file, err := os.OpenFile(server_config.Logging.AccessLogFile, os.O_APPEND|os.O_RDWR, 0666)
			if err != nil {
				log.Fatal("Log: OpenFile: ", err.Error())
			}
			t.AccessLog = apachelog.NewApacheLog(log_file, apache_log_format)
		}
	} else {
		t.AccessLog = apachelog.NewApacheLog(os.Stderr, apache_log_format)
	}
}

func (t *ApiHandler) AddServlet(endpoint string, handler http.Handler) {
	t.Servlets[endpoint] = handler.ServeHTTP
}

func (t *ApiHandler) AddServletFunc(endpoint string, handler func(http.ResponseWriter, *http.Request)) {
	t.Servlets[endpoint] = handler
}

func (t *ApiHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	lw := apachelog.NewLoggingWriter(w, r, t.AccessLog)
	defer lw.EmitLog()

	if servlet, servlet_exists := t.Servlets[r.RequestURI]; servlet_exists {
		servlet(w, r)
	} else {
		ServeError(w, r, fmt.Sprintf("No matching servlet for request %s", r.RequestURI), 404)
	}
}

func ServeError(w http.ResponseWriter, r *http.Request, error string, errcode int) {
	error_struct := ApiError{
		Success: 0,
		Error:   error}
	error_json, err := json.Marshal(error_struct)
	if err != nil {
		log.Println(err)
		http.Error(w, "Internal server error", 500)
		return
	}
	http.Error(w, string(error_json), errcode)
}
