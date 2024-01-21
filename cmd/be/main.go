package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
)

func main() {
	// Define a command-line flag for the port
	var port int
	flag.IntVar(&port, "port", 3001, "Port for the server to listen on")

	// Parse the command-line arguments
	flag.Parse()

	// Define a handler function to handle incoming HTTP requests
	handler := func(w http.ResponseWriter, r *http.Request) {
		// Print details of the incoming request
		fmt.Printf("Received request from %s\n", r.RemoteAddr)
		fmt.Printf("%s %s %s\n", r.Method, r.URL, r.Proto)
		fmt.Println("Host:", r.Host)
		fmt.Println("User-Agent:", r.UserAgent())
		fmt.Println("Accept:", r.Header.Get("Accept"))
		fmt.Println("Replied with a hello message")

		// Set the Content-Type header
		w.Header().Set("Content-Type", "text/plain")

		// Write the response
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello from backend server!"))
	}

	// Use the http.HandleFunc function to associate the handler with a route (in this case, the root '/')
	http.HandleFunc("/", handler)

	// Start the web server in a goroutine
	go func() {
		err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
		if err != nil {
			fmt.Printf("Error starting the server: %s\n", err)
			os.Exit(1)
		}
	}()

	fmt.Printf("Server is listening on port %d...\n", port)

	// Keep the main goroutine running to allow the server to continue in the background
	select {}
}
