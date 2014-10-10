package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var server_config Config

var http_server http.Server

func init_server() {

	bind_address := server_config.Network.BindAddress + ":" + server_config.Network.BindPort

	api_handler := NewApiHandler()

	read_timeout, err := time.ParseDuration(server_config.Network.ReadTimeout)
	if err != nil {
		read_timeout = 10 * time.Second
		log.Println("Invalid network timeout", server_config.Network.ReadTimeout)
	}

	write_timeout, err := time.ParseDuration(server_config.Network.WriteTimeout)
	if err != nil {
		write_timeout = 10 * time.Second
		log.Println("Invalid network timeout", server_config.Network.WriteTimeout)
	}

	http_server = http.Server{
		Addr:           bind_address,
		Handler:        api_handler,
		ReadTimeout:    read_timeout,
		WriteTimeout:   write_timeout,
		MaxHeaderBytes: 1 << 20,
	}

	api_handler.AddServletFunc("/version", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "{ \"success\":1, \"return\":{ \"build\":\"%s\"} }", build_version)
	})

	// Start listening to HTTP requests
	if err := http_server.ListenAndServe(); err != nil {
		log.Fatalln("Fatal Error: ListenAndServe: ", err.Error())
	}

	log.Println("Listening on " + bind_address)
}

func main() {

	log.Println("Starting API server")

	/*
	 * Load Configuration
	 */
	server_config = LoadConfiguration("/opt/degree/server.gcfg")

	/*
	 * Set up log facility
	 */
	if _, err := os.Stat(server_config.Logging.LogFile); os.IsNotExist(err) {
		log_file, err := os.Create(server_config.Logging.LogFile)
		if err != nil {
			log.Fatal("Log: Create: ", err.Error())
		}
		log.SetOutput(log_file)
	} else {
		log_file, err := os.OpenFile(server_config.Logging.LogFile, os.O_APPEND|os.O_RDWR, 0666)
		if err != nil {
			log.Fatal("Log: OpenFile: ", err.Error())
		}
		log.SetOutput(log_file)
	}

	/*
	 * Set up signal handlers
	 */

	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		for sig := range c {
			log.Println("Exiting with signal", sig)
			os.Exit(1)
		}
	}()

	/*
	 * Start server
	 */
	init_server()
}
