package main

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"
)

// Backend defines the interface for a backend server
type Backend interface {
	ServeHTTP(w http.ResponseWriter, r *http.Request)
	SetAlive(alive bool)
	IsAlive() bool
	GetURL() *url.URL
	GetActiveConnections() int
	PerformHealthCheck(interval time.Duration)
}

// backend is a simple round-robin load balancer
type backend struct {
	URL               *url.URL
	alive             bool
	activeConnections int
	mutex             sync.RWMutex
	reverseProxy      *httputil.ReverseProxy
	healthCheckURL    string
}

func NewBackend(URL string) Backend {
	u, err := url.Parse(URL)
	if err != nil {
		panic(err)
	}

	return &backend{
		URL:            u,
		alive:          true,
		reverseProxy:   httputil.NewSingleHostReverseProxy(u),
		healthCheckURL: URL + "/health", // Assuming a simple health check endpoint at /health
	}
}

func (b *backend) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Print details of the incoming request
	fmt.Printf("Received request from %s\n", r.RemoteAddr)
	fmt.Printf("%s %s %s\n", r.Method, r.URL, r.Proto)
	fmt.Println("Host:", r.Host)
	fmt.Println("User-Agent:", r.UserAgent())
	fmt.Println("Accept:", r.Header.Get("Accept"))

	// Set the Host header for the outgoing request
	r.Host = b.URL.Host

	// Forward the request to the backend server
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	if b.alive {
		b.activeConnections++

		b.reverseProxy.ServeHTTP(w, r)
		b.activeConnections--
	} else {
		http.Error(w, "Backend server is not available", http.StatusServiceUnavailable)
	}
}

func (b *backend) SetAlive(alive bool) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.alive = alive
}

func (b *backend) IsAlive() bool {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return b.alive
}

func (b *backend) GetURL() *url.URL {
	return b.URL
}

func (b *backend) GetActiveConnections() int {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return b.activeConnections
}

// PerformHealthCheck periodically checks if the backend server is alive
func (b *backend) PerformHealthCheck(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := b.checkHealth(); err != nil {
				fmt.Printf("Health check failed for %s: %s\n", b.healthCheckURL, err)
				b.SetAlive(false)
			} else {
				fmt.Printf("Health check passed for %s\n", b.healthCheckURL)
				b.SetAlive(true)
			}
		}
	}
}

func (b *backend) checkHealth() error {
	resp, err := http.Get(b.healthCheckURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

// ServerPool represents a pool of backend servers
type ServerPool interface {
	GetBackends() []Backend
	GetNextValidPeer() Backend
	AddBackend(Backend)
	GetServerPoolSize() int
}

// RoundRobinServerPool represents a pool of backend servers using round-robin selection
type RoundRobinServerPool struct {
	backends []Backend
	index    int
	mutex    sync.RWMutex
}

// NewRoundRobinServerPool creates a new RoundRobinServerPool instance
func NewRoundRobinServerPool() *RoundRobinServerPool {
	return &RoundRobinServerPool{
		backends: make([]Backend, 0),
	}
}

// GetBackends returns the list of backend servers in the pool
func (sp *RoundRobinServerPool) GetBackends() []Backend {
	sp.mutex.RLock()
	defer sp.mutex.RUnlock()
	return sp.backends
}

// GetNextValidPeer returns the next available backend server in a round-robin fashion
func (sp *RoundRobinServerPool) GetNextValidPeer() Backend {
	sp.mutex.Lock()
	defer sp.mutex.Unlock()

	for range sp.backends {
		backend := sp.backends[sp.index]
		sp.index = (sp.index + 1) % len(sp.backends)

		if backend.IsAlive() {
			return backend
		}
	}

	return nil
}

// AddBackend adds a backend server to the pool
func (sp *RoundRobinServerPool) AddBackend(backend Backend) {
	sp.mutex.Lock()
	defer sp.mutex.Unlock()
	sp.backends = append(sp.backends, backend)

	// Start health check for the new backend
	go backend.PerformHealthCheck(10 * time.Second) // Adjust the interval as needed
}

// GetServerPoolSize returns the number of backend servers in the pool
func (sp *RoundRobinServerPool) GetServerPoolSize() int {
	sp.mutex.RLock()
	defer sp.mutex.RUnlock()
	return len(sp.backends)
}

func main() {
	// Create a new RoundRobinServerPool
	serverPool := NewRoundRobinServerPool()

	// Create two Backend instances representing backend servers
	backend1 := NewBackend("http://localhost:3001")
	backend2 := NewBackend("http://localhost:3002")

	// Add the backends to the server pool
	serverPool.AddBackend(backend1)
	serverPool.AddBackend(backend2)

	// Use the ServerPool as the handler for incoming requests
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		peer := serverPool.GetNextValidPeer()

		fmt.Printf("Selected peer at %s\n", peer.GetURL())

		if peer != nil {
			peer.ServeHTTP(w, r)
			fmt.Println("Response from backend server")
		} else {
			http.Error(w, "No backend server is available", http.StatusServiceUnavailable)
		}
	})

	// Specify the port number to listen on
	port := 3000

	// Start the load balancer server
	go func() {
		err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
		if err != nil {
			fmt.Printf("Error starting the load balancer: %s\n", err)
		}
	}()

	fmt.Printf("Load balancer started on port %d\n", port)
	select {}
}
