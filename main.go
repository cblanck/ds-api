package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

var server_config Config

func init_server() {
	bind_address := server_config.Network.BindAddress + ":" + server_config.Network.BindPort

	http.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Build %s", build_version)
	})

	// Start listening to HTTP requests
	if err := http.ListenAndServe(bind_address, nil); err != nil {
		log.Fatalln("Fatal Error: ListenAndServe: ", err.Error())
	}

	log.Println("Listening on " + bind_address)
}

func main() {

	log.Println("Starting API server")

	/*
	 * Load Configuration
	 */
	server_config = LoadConfiguration("server.gcfg")

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
