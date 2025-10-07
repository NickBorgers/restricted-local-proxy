package main

import (
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"time"
)

// HARDCODED ALLOWLIST - this is compiled into the binary
// Format: "hostname:port" or just "hostname" (any port)
var allowlist = []string{
	"example.com",
	"httpbin.org",
	"www.google.com:443",
	"api.github.com:443",
}

// ProxyServer handles HTTP CONNECT requests for tunneling
type ProxyServer struct {
	allowlist map[string]bool
	port      string
}

// NewProxyServer creates a new proxy server with the hardcoded allowlist
func NewProxyServer(port string) *ProxyServer {
	allowMap := make(map[string]bool)
	for _, entry := range allowlist {
		allowMap[entry] = true
	}
	return &ProxyServer{
		allowlist: allowMap,
		port:      port,
	}
}

// isAllowed checks if a host:port combination is allowed
func (p *ProxyServer) isAllowed(hostPort string) bool {
	// Check exact match first (host:port)
	if p.allowlist[hostPort] {
		return true
	}

	// Check if just the hostname is in allowlist (allows any port)
	host, _, err := net.SplitHostPort(hostPort)
	if err == nil && p.allowlist[host] {
		return true
	}

	return false
}

// handleConnect handles HTTP CONNECT method for HTTPS tunneling
func (p *ProxyServer) handleConnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodConnect {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	destHost := r.Host

	// Check allowlist
	if !p.isAllowed(destHost) {
		log.Printf("BLOCKED: Connection attempt to %s (not in allowlist)", destHost)
		http.Error(w, "Forbidden: Destination not allowed", http.StatusForbidden)
		return
	}

	log.Printf("ALLOWED: Connecting to %s", destHost)

	// Connect to the destination
	destConn, err := net.DialTimeout("tcp", destHost, 10*time.Second)
	if err != nil {
		log.Printf("ERROR: Failed to connect to %s: %v", destHost, err)
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
		return
	}
	defer destConn.Close()

	// Hijack the client connection
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer clientConn.Close()

	// Send 200 Connection Established to client
	clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))

	// Bidirectional copy between client and destination
	var wg sync.WaitGroup
	wg.Add(2)

	// Client -> Destination
	go func() {
		defer wg.Done()
		io.Copy(destConn, clientConn)
		destConn.Close()
	}()

	// Destination -> Client
	go func() {
		defer wg.Done()
		io.Copy(clientConn, destConn)
		clientConn.Close()
	}()

	wg.Wait()
	log.Printf("Connection to %s closed", destHost)
}

// Start starts the proxy server
func (p *ProxyServer) Start() error {
	server := &http.Server{
		Addr:    ":" + p.port,
		Handler: http.HandlerFunc(p.handleConnect),
	}

	log.Printf("Restricted Local Proxy starting on port %s", p.port)
	log.Printf("Allowlist contains %d entries:", len(allowlist))
	for _, entry := range allowlist {
		log.Printf("  - %s", entry)
	}

	return server.ListenAndServe()
}

func main() {
	port := "9091"
	proxy := NewProxyServer(port)

	if err := proxy.Start(); err != nil {
		log.Fatalf("Proxy server failed: %v", err)
	}
}
