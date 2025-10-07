package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"

	"gopkg.in/yaml.v3"
)

// LogEntry matches the structure from main.go
type DiscoveryLogEntry struct {
	Timestamp   string `json:"timestamp"`
	Level       string `json:"level"`
	Event       string `json:"event"`
	Destination string `json:"destination,omitempty"`
	Action      string `json:"action,omitempty"`
}

// Config represents the YAML output structure
type OutputConfig struct {
	Allowlist []string `yaml:"allowlist"`
}

func main() {
	inputFile := flag.String("input", "", "Input log file (JSON lines format)")
	outputFile := flag.String("output", "allowlist.yaml", "Output YAML config file")
	flag.Parse()

	if *inputFile == "" {
		fmt.Fprintf(os.Stderr, "Usage: logs-to-config -input <logfile> [-output <yamlfile>]\n")
		os.Exit(1)
	}

	// Open input file
	file, err := os.Open(*inputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening input file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	// Track unique destinations
	destinations := make(map[string]bool)

	// Read log file line by line
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		var entry DiscoveryLogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			// Skip malformed lines
			continue
		}

		// Only process connection attempts
		if entry.Event == "connection_attempt" && entry.Destination != "" {
			destinations[entry.Destination] = true
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading log file: %v\n", err)
		os.Exit(1)
	}

	// Convert to sorted slice
	var allowlist []string
	for dest := range destinations {
		allowlist = append(allowlist, dest)
	}
	sort.Strings(allowlist)

	// Create YAML config
	config := OutputConfig{
		Allowlist: allowlist,
	}

	// Write YAML file
	data, err := yaml.Marshal(&config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling YAML: %v\n", err)
		os.Exit(1)
	}

	// Add header comment
	header := "# Allowlist configuration generated from discovery logs\n# Format: hostname:port or just hostname (allows any port)\n"
	output := []byte(header)
	output = append(output, data...)

	if err := os.WriteFile(*outputFile, output, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing output file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Generated %s with %d unique destinations\n", *outputFile, len(allowlist))
	for _, dest := range allowlist {
		fmt.Printf("  - %s\n", dest)
	}
}
