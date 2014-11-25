package main

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"github.com/bradfitz/gomemcache/memcache"
	"github.com/rschlaikjer/go-apache-logformat"
	"log"
	"net/http"
	"os"
	"reflect"
	"strings"
)

const apache_log_format = `%h %l %u %t "%r" %>s %b "%{Referer}i" "%{User-agent}i"`

type ApiError struct {
	Success int
	Error   string
}

type ApiSuccess struct {
	Success int
	Return  interface{}
}

type Servlet interface{}

type ApiHandler struct {
	Servlets  map[string]Servlet
	AccessLog *apachelog.ApacheLog
	Memcached *memcache.Client
}

func NewApiHandler(server_config *Config) *ApiHandler {
	h := new(ApiHandler)
	h.SetAccessLog(server_config)
	h.Servlets = make(map[string]Servlet)
	h.Memcached = memcache.New(server_config.Memcache.Host)
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

func (t *ApiHandler) AddServlet(endpoint string, handler Servlet) {
	t.Servlets[endpoint] = handler
}

func GenerateMemcacheHash(servlet string, args map[string][]string) string {
	// Generates a memcached key value for the calling function based on the
	// key:value pairs passed in the HTTP form

	// Concatenate and hash the argmap
	joined_args := ""
	for key, value := range args {
		// Don't hash on session
		if key == "session" {
			continue
		}
		if len(value) == 0 {
			continue
		}
		joined_args = fmt.Sprintf("%s:%s,%s", key, value[0], joined_args)
	}
	arg_hash := sha1.Sum([]byte(joined_args))

	// Generate the memcached key
	memcached_key := fmt.Sprintf(
		"%s-%x",
		servlet,
		arg_hash,
	)

	return memcached_key
}

// Store the result of an API request in memcached.
// Should not be used for generics, as it encapsulates values in a ApiSuccess
// struct before converting them to JSON.
func SetCachedRequest(m *memcache.Client, key string, value interface{}) {
	result_struct := ApiSuccess{
		Success: 1,
		Return:  value,
	}
	result_json, err := json.MarshalIndent(result_struct, "", "  ")
	if err != nil {
		log.Println(err)
	}
	m.Set(
		&memcache.Item{
			Key:        key,
			Value:      result_json,
			Expiration: 600,
		},
	)
}

// Fetches a previously cached API request from memcached, or returns false if
// none exists. Values should only have been set using the CacheSetRequest
// method, and so should be value JSON consisting of a result encapsulated by
// an ApiResult struct.
func GetCachedRequest(memcache *memcache.Client, key string) (bool, []byte) {
	// Try and retrieve the serialized data
	cached_json, err := memcache.Get(key)
	if err != nil {
		// Errors are cache misses
		return false, nil
	}
	return true, cached_json.Value
}

// Deals with incoming HTTP requests. Checks if the appropriate servlet exists,
// and if so gets the servlet method to handle the request.
// If the method is cacheable, also deal with getting/setting cached values.
func (t *ApiHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	lw := apachelog.NewLoggingWriter(w, r, t.AccessLog)
	defer lw.EmitLog()

	if servlet, servlet_exists := t.Servlets[r.RequestURI]; servlet_exists {
		r.ParseForm()
		method := r.Form.Get("method")

		// Try and get a pointer to the handler method for the request
		// If no handler exists, fail with a Bad Request message.
		method_handler, method_cacheable := GetMethodForRequest(servlet, method)
		if method_handler == nil {
			ServeError(w, r,
				fmt.Sprintf("Servlet %s No such method '%s'", r.RequestURI, method),
				400)
			return
		}

		// If the method is cacheable, try and fetch a cached version
		var mc_key string
		if method_cacheable {
			mc_key = GenerateMemcacheHash(r.RequestURI, r.Form)
			if request_cached, cached_value := GetCachedRequest(t.Memcached, mc_key); request_cached {
				ServeRawResult(w, r, cached_value)
				return
			}
		}

		// Perform the method call and unpack the reflect.Value response
		// The prototype for the method returns a single interface{}
		args := make([]reflect.Value, 1)
		args[0] = reflect.ValueOf(r)
		response_value := method_handler.Call(args)
		var response_data interface{} = nil
		if len(response_value) == 1 {
			response_data = response_value[0].Interface
		}

		if response_value != nil {
			if method_cacheable {
				SetCachedRequest(t.Memcached, mc_key, response_value)
			}
			ServeResult(w, r, response_value)
		} else {
			ServeError(w, r, "Internal Server Error", 500)
		}
	} else {
		ServeError(w, r, fmt.Sprintf("No matching servlet for request %s", r.RequestURI), 404)
	}
}

func ServeError(w http.ResponseWriter, r *http.Request, error string, errcode int) {
	error_struct := ApiError{
		Success: 0,
		Error:   error}
	error_json, err := json.MarshalIndent(error_struct, "", "  ")
	if err != nil {
		log.Println(err)
		http.Error(w, "Internal server error", 500)
		return
	}
	http.Error(w, string(error_json), errcode)
}

func ServeResult(w http.ResponseWriter, r *http.Request, result interface{}) {
	result_struct := ApiSuccess{
		Success: 1,
		Return:  result}
	result_json, err := json.MarshalIndent(result_struct, "", "  ")
	if err != nil {
		log.Println(err)
		http.Error(w, "Internal server error", 500)
		return
	}
	fmt.Fprintf(w, string(result_json))
}

// Write a raw JSON result, e.g. the return of CacheGetRequest
func ServeRawResult(w http.ResponseWriter, r *http.Request, result []byte) {
	fmt.Fprintf(w, "%s", result)
}

// To avoid a massive case statement, use reflection to do a lookup of the given
// method on the servlet. MethodByName will return a 'Zero Value' for methods
// that aren't found, which will return false for .IsValid.
// Performing Call() on an unexported method is a runtime violation, uppercasing
// the first letter in the method name before reflection avoids locating
// unexported functions. A little hacky, but it works.
// This method also determines whether a given method can be cached, based again
// on the name that is given to the method. Methods prefixes with Cacheable will
// be reported as such.
//
// For more info, see http://golang.org/pkg/reflect/
func GetMethodForRequest(t interface{}, method string) (*reflect.Value, bool) {
	if method == "" {
		method = "ServeHTTP"
	}

	upper_method := strings.ToUpper(method)
	exported_method := []byte(method)
	exported_method[0] = upper_method[0]

	// Check if the method is raw (not cacheable)
	servlet_value := reflect.ValueOf(t)
	method_handler := servlet_value.MethodByName(string(exported_method))
	if method_handler.IsValid() {
		return &method_handler, false
	}

	// Check if the meythod exists and is cacheable
	cacheable_method_name := fmt.Sprintf("Cacheable%s", exported_method)
	method_handler = servlet_value.MethodByName(string(cacheable_method_name))
	if method_handler.IsValid() {
		return &method_handler, true
	}

	return nil, false
}
