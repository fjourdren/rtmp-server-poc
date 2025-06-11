package config

import "time"

// Config holds all configuration for the application
type Config struct {
	// Server configuration
	RTMPPort string
	HTTPPort string
	
	// Output configuration
	OutputDir string
	
	// Stream configuration
	ReconnectDelay time.Duration
	CleanupDelay   time.Duration
	
	// Authorization configuration
	AuthorizedPatterns []string
}

// DefaultConfig returns a default configuration
func DefaultConfig() Config {
	return Config{
		RTMPPort: ":1935",
		HTTPPort: ":8080",
		OutputDir: "./streams",
		ReconnectDelay: 5 * time.Second,
		CleanupDelay: 2 * time.Second,
		AuthorizedPatterns: []string{
			"/live/{app}/{username}",
		},
	}
} 