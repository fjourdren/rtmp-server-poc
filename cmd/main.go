// A complete runnable example that:
//  1. Manages multiple streamers simultaneously with multi-threading
//  2. Each streamer gets their own FFmpeg process and output directory
//  3. URL structure: /stream/{username}/live.m3u8 for viewing
//  4. RTMP publish with authorization: rtmp://localhost/live/{app}/{username}
//  5. Path-based pattern matching for flexible authorization
//  6. Play connections are blocked (publish-only server)
//
// Build & run:
//
//	go run ./cmd/main.go
//
// Publish with OBS (or any RTMP encoder):
//
//	rtmp://localhost/live/test/johndoe
//	rtmp://localhost/live/myapp/alice
//
// Watch streams at:
//
//	http://localhost:8080/stream/{username}/live.m3u8
//
// Authorization patterns (configurable in AUTHORIZED_APPS_TCURLS):
//
//	/live/{app}/{username} - matches rtmp://anyhost/live/anyapp/anyuser
//
// -----------------------------------------------------------------------------
package main

import (
	"io"
	"log"
	"net"
	"os"

	"github.com/yutopp/go-rtmp"

	"rtmp-server-poc/internal/config"
	httpserver "rtmp-server-poc/internal/http"
	rtmphandler "rtmp-server-poc/internal/rtmp"
	"rtmp-server-poc/internal/stream"
)

func main() {
	// Load configuration
	cfg := config.DefaultConfig()

	// Create output directory
	if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
		log.Fatal(err)
	}

	// Create stream manager
	streamManager := stream.NewManager()

	// Start HTTP server
	httpSrv := httpserver.NewServer(cfg, streamManager)
	httpServer := httpSrv.SetupServer()
	go func() {
		log.Printf("HTTP server listening on %s", cfg.HTTPPort)
		log.Println("Visit http://localhost:8080 to see active streams")
		if err := httpServer.ListenAndServe(); err != nil {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	// Start RTMP server
	srv := rtmp.NewServer(&rtmp.ServerConfig{
		OnConnect: func(conn net.Conn) (io.ReadWriteCloser, *rtmp.ConnConfig) {
			return conn, &rtmp.ConnConfig{
				Handler: rtmphandler.NewHandler(streamManager, cfg),
			}
		},
	})

	ln, err := net.Listen("tcp", cfg.RTMPPort)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("RTMP server listening on %s", cfg.RTMPPort)
	log.Println("Publish streams to: rtmp://localhost/live/{app}/{username}")
	log.Println("Watch streams at: http://localhost:8080/stream/{username}/live.m3u8")

	if err := srv.Serve(ln); err != nil {
		log.Fatal(err)
	}
}
