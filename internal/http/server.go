package http

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"rtmp-server-poc/internal/config"
	"rtmp-server-poc/internal/stream"
)

// Server handles HTTP requests for HLS streaming
type Server struct {
	config        config.Config
	streamManager *stream.Manager
}

// NewServer creates a new HTTP server
func NewServer(cfg config.Config, manager *stream.Manager) *Server {
	return &Server{
		config:        cfg,
		streamManager: manager,
	}
}

// SetupServer sets up the HTTP server for HLS streaming
func (s *Server) SetupServer() *http.Server {
	mux := http.NewServeMux()

	// Stream handler
	mux.HandleFunc("/stream/", s.handleStreamRequest)

	// Root handler (stream list)
	mux.HandleFunc("/", s.handleRootRequest)

	return &http.Server{
		Addr:    s.config.HTTPPort,
		Handler: mux,
	}
}

// handleStreamRequest handles requests for stream files
func (s *Server) handleStreamRequest(w http.ResponseWriter, r *http.Request) {
	// Add CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type")

	if r.Method == "OPTIONS" {
		return
	}

	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 2 {
		http.NotFound(w, r)
		return
	}

	username := pathParts[1]
	if username == "" {
		http.NotFound(w, r)
		return
	}

	streamDir := filepath.Join(s.config.OutputDir, username)
	if _, err := os.Stat(streamDir); os.IsNotExist(err) {
		http.NotFound(w, r)
		return
	}

	remainingPath := strings.Join(pathParts[2:], "/")
	if remainingPath == "" {
		remainingPath = "live.m3u8"
	}

	filePath := filepath.Join(streamDir, remainingPath)

	if filepath.Ext(filePath) == ".m3u8" || filepath.Ext(filePath) == ".ts" {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
	}

	http.ServeFile(w, r, filePath)
}

// handleRootRequest handles requests to the root path
func (s *Server) handleRootRequest(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// Collect active streams
	activeStreams := s.streamManager.GetActiveStreams()

	// Render HTML
	w.Header().Set("Content-Type", "text/html")
	s.renderStreamList(w, activeStreams)
}

// renderStreamList renders the HTML page with the list of active streams
func (s *Server) renderStreamList(w io.Writer, activeStreams []string) {
	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
    <title>RTMP2HLS Server</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        .stream-list { margin-top: 20px; }
        .stream-link { display: block; margin: 10px 0; padding: 10px; background: #f0f0f0; text-decoration: none; color: #333; border-radius: 5px; }
        .stream-link:hover { background: #e0e0e0; }
        .instructions { background: #f9f9f9; padding: 20px; border-radius: 5px; margin-bottom: 20px; }
        .code { background: #f0f0f0; padding: 5px; border-radius: 3px; font-family: monospace; }
    </style>
</head>
<body>
    <h1>RTMP2HLS Multi-Streamer Server</h1>
    
    <div class="instructions">
        <h3>How to use:</h3>
        <p><strong>Publish streams (with authorization):</strong></p>
        <p><span class="code">rtmp://localhost/live/{app}/{username}</span></p>
        <p><strong>Examples:</strong></p>
        <ul>
            <li><span class="code">rtmp://localhost/live/test/johndoe</span></li>
            <li><span class="code">rtmp://localhost/live/myapp/alice</span></li>
        </ul>
        <p><strong>Watch streams:</strong></p>
        <p><span class="code">http://localhost:8080/stream/{username}/live.m3u8</span></p>
        <p><strong>Note:</strong> Play connections are blocked. This is a publish-only server.</p>
    </div>

    <h2>Active Streams (%d)</h2>
    <div class="stream-list">`, len(activeStreams))

	if len(activeStreams) == 0 {
		fmt.Fprintf(w, `<p>No active streams currently.</p>`)
	} else {
		for _, username := range activeStreams {
			fmt.Fprintf(w, `<a href="/stream/%s/live.m3u8" class="stream-link">%s - Click to view stream</a>`, username, username)
		}
	}

	fmt.Fprintf(w, `    </div>
</body>
</html>`)
} 