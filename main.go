package main

import (
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

//go:embed allowlist.yaml
var allowlistYAML []byte

// Config represents the YAML configuration structure
type Config struct {
	Allowlist []string `yaml:"allowlist"`
}

// DiscoveryMode is set at compile time using -ldflags "-X main.DiscoveryMode=true"
var DiscoveryMode = "false"

// loadAllowlist loads and parses the embedded YAML configuration
func loadAllowlist() ([]string, error) {
	var config Config
	if err := yaml.Unmarshal(allowlistYAML, &config); err != nil {
		return nil, fmt.Errorf("failed to parse allowlist.yaml: %w", err)
	}
	return config.Allowlist, nil
}

// LogLevel represents the severity of a log message
type LogLevel string

const (
	LogLevelInfo    LogLevel = "INFO"
	LogLevelWarning LogLevel = "WARNING"
	LogLevelError   LogLevel = "ERROR"
	LogLevelDebug   LogLevel = "DEBUG"
)

// LogEntry represents a structured log entry
type LogEntry struct {
	Timestamp    string                 `json:"timestamp"`
	Level        LogLevel               `json:"level"`
	Event        string                 `json:"event"`
	Destination  string                 `json:"destination,omitempty"`
	Action       string                 `json:"action,omitempty"`
	Error        string                 `json:"error,omitempty"`
	AllowedCount int                    `json:"allowed_count,omitempty"`
	Message      string                 `json:"message,omitempty"`
	Extra        map[string]interface{} `json:"extra,omitempty"`
}

// Logger handles structured logging
type Logger struct {
	output io.Writer
}

// NewLogger creates a new structured logger
func NewLogger(output io.Writer) *Logger {
	return &Logger{output: output}
}

// Log writes a structured log entry
func (l *Logger) Log(entry LogEntry) {
	entry.Timestamp = time.Now().UTC().Format(time.RFC3339)
	data, err := json.Marshal(entry)
	if err != nil {
		log.Printf("Failed to marshal log entry: %v", err)
		return
	}
	fmt.Fprintln(l.output, string(data))
}

// Info logs an info-level message
func (l *Logger) Info(event, message string) {
	l.Log(LogEntry{Level: LogLevelInfo, Event: event, Message: message})
}

// Error logs an error-level message
func (l *Logger) Error(event, message, errMsg string) {
	l.Log(LogEntry{Level: LogLevelError, Event: event, Message: message, Error: errMsg})
}

// ConnectionAttempt logs a connection attempt
func (l *Logger) ConnectionAttempt(destination, action string, err error) {
	entry := LogEntry{
		Level:       LogLevelInfo,
		Event:       "connection_attempt",
		Destination: destination,
		Action:      action,
	}
	if err != nil {
		entry.Level = LogLevelError
		entry.Error = err.Error()
	}
	l.Log(entry)
}

// ProxyServer handles HTTP CONNECT requests for tunneling
type ProxyServer struct {
	allowlist     map[string]bool
	listen        string
	discoveryMode bool
	logger        *Logger
}

// NewProxyServer creates a new proxy server with the embedded YAML allowlist
func NewProxyServer(listen string, logger *Logger) (*ProxyServer, error) {
	allowlistEntries, err := loadAllowlist()
	if err != nil {
		return nil, err
	}

	allowMap := make(map[string]bool)
	for _, entry := range allowlistEntries {
		allowMap[entry] = true
	}

	discoveryMode := DiscoveryMode == "true"

	return &ProxyServer{
		allowlist:     allowMap,
		listen:        listen,
		discoveryMode: discoveryMode,
		logger:        logger,
	}, nil
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

	// In discovery mode, allow all connections and log them
	if p.discoveryMode {
		p.logger.ConnectionAttempt(destHost, "allowed_discovery", nil)
	} else {
		// Check allowlist in normal mode
		if !p.isAllowed(destHost) {
			p.logger.ConnectionAttempt(destHost, "blocked", nil)
			http.Error(w, "Forbidden: Destination not allowed", http.StatusForbidden)
			return
		}
		p.logger.ConnectionAttempt(destHost, "allowed", nil)
	}

	// Connect to the destination
	destConn, err := net.DialTimeout("tcp", destHost, 10*time.Second)
	if err != nil {
		p.logger.ConnectionAttempt(destHost, "connection_failed", err)
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
	p.logger.Log(LogEntry{
		Level:       LogLevelInfo,
		Event:       "connection_closed",
		Destination: destHost,
	})
}

// Start starts the proxy server
func (p *ProxyServer) Start() error {
	server := &http.Server{
		Addr:    p.listen,
		Handler: http.HandlerFunc(p.handleConnect),
	}

	mode := "RESTRICTED"
	if p.discoveryMode {
		mode = "DISCOVERY"
	}

	p.logger.Log(LogEntry{
		Level:        LogLevelInfo,
		Event:        "proxy_starting",
		Message:      fmt.Sprintf("Mode: %s, Listen: %s", mode, p.listen),
		AllowedCount: len(p.allowlist),
	})

	// Log allowlist entries
	for entry := range p.allowlist {
		p.logger.Log(LogEntry{
			Level:       LogLevelDebug,
			Event:       "allowlist_entry",
			Destination: entry,
		})
	}

	return server.ListenAndServe()
}

func main() {
	// Command line flags
	listen := flag.String("listen", "localhost:9091", "Address to listen on (e.g., localhost:9091 or :8080)")
	flag.Parse()

	logger := NewLogger(os.Stdout)

	proxy, err := NewProxyServer(*listen, logger)
	if err != nil {
		logger.Error("initialization_failed", "Failed to create proxy server", err.Error())
		os.Exit(1)
	}

	if err := proxy.Start(); err != nil {
		logger.Error("server_failed", "Proxy server failed", err.Error())
		os.Exit(1)
	}
}
