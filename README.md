# RTMP to HLS Streaming Server

**This is a Proof of Concept (POC) implementation of a multi-streamer RTMP to HLS conversion server with authorization support.**

A multi-streamer RTMP to HLS conversion server with authorization support. This server accepts RTMP streams from multiple publishers, converts them to HLS format using FFmpeg, and serves them via HTTP.

## Architecture Overview

The server consists of several key components that work together to handle the complete stream lifecycle:

### Core Components

1. **RTMP Server** - Accepts incoming RTMP connections
2. **Authorization System** - Validates stream URLs against authorized patterns
3. **Stream Manager** - Manages multiple concurrent streams
4. **FFmpeg Processes** - Converts RTMP/FLV to HLS
5. **HTTP Server** - Serves HLS streams to viewers
6. **FLV Writer** - Handles FLV tag writing to FFmpeg

## Stream Lifecycle: Complete Process

### 1. Server Startup

When the server starts, the following objects are created:

```go
// Main application objects
config := config.DefaultConfig()           // Configuration with ports, paths, auth patterns
streamManager := stream.NewManager()       // Manages all active streams
httpServer := httpserver.NewServer()       // HTTP server for HLS delivery
rtmpServer := rtmp.NewServer()             // RTMP server for incoming streams
```

**Configuration Object (`config.Config`):**
- `RTMPPort`: ":1935" (RTMP server port)
- `HTTPPort`: ":8080" (HTTP server port)
- `OutputDir`: "./out" (HLS output directory)
- `AuthorizedPatterns`: ["/live/{app}/{username}"] (URL patterns for authorization)
- `ReconnectDelay`: 5s (delay before cleanup after disconnect)
- `CleanupDelay`: 2s (delay for cleanup operations)

### 2. RTMP Connection Establishment

When a client connects to `rtmp://localhost/live/test/johndoe`:

```go
// RTMP Handler creation (one per connection)
handler := rtmphandler.NewHandler(streamManager, config)
// Creates:
// - streamManager reference
// - config reference  
// - authorizer := auth.NewAuthorizer(config.AuthorizedPatterns)
// - connectionInfo := nil (will be set in OnConnect)
// - streamProcess := nil (will be set in OnPublish)
// - flvWriter := nil (will be set in OnPublish)
```

### 3. Connection Authorization (`OnConnect`)

The server validates the incoming TCURL against authorized patterns:

```go
// Extract variables from TCURL using pattern matching
vars, ok := authorizer.ExtractVariables("rtmp://localhost/live/test/johndoe")
// Returns: map[string]string{"app": "test", "username": "johndoe"}

// Store connection information
connectionInfo := &models.ConnectionInfo{
    App:   "live",
    TCURL: "rtmp://localhost/live/test/johndoe", 
    Vars:  vars, // {"app": "test", "username": "johndoe"}
}
```

**ConnectionInfo Object:**
- `App`: The RTMP application name ("live")
- `TCURL`: The complete target URL
- `Vars`: Extracted variables from pattern matching

### 4. Stream Publishing (`OnPublish`)

When the client starts publishing the stream:

```go
// Validate authentication (username must match publishing name)
err := authorizer.ValidateAuthentication(vars, "johndoe")

// Get or create stream process
streamProcess := streamManager.GetOrCreateStream("johndoe", config)
```

**StreamProcess Object Creation:**
```go
streamProcess := &StreamProcess{
    username:  "johndoe",
    cmd:       *exec.Cmd,           // FFmpeg process
    stdin:     io.WriteCloser,      // Pipe to FFmpeg stdin
    cancel:    context.CancelFunc,  // Function to cancel FFmpeg
    outputDir: "./streams/johndoe",
    active:    atomic.Bool{true},   // Thread-safe active state
}
```

**FFmpeg Process Creation:**
```bash
ffmpeg -re -fflags +nobuffer -flags low_delay -f flv -i pipe:0 \
       -c:v copy -c:a copy -f hls -hls_time 1 -hls_list_size 3 \
       -hls_flags delete_segments+temp_file+independent_segments \
       -hls_segment_type mpegts -hls_allow_cache 0 \
       -hls_segment_filename ./streams/johndoe/live_%03d.ts \
       ./streams/johndoe/live.m3u8
```

**FLV Writer Object:**
```go
flvWriter := &flv.Writer{
    writer:     streamProcess.stdin,  // Writes to FFmpeg stdin
    writeMutex: sync.Mutex{},         // Thread-safe writing
    headerOnce: sync.Once{},          // FLV header written once
}
```

### 5. Stream Data Flow

During streaming, data flows through the system:

```
RTMP Client → RTMP Handler → FLV Writer → FFmpeg Process → HLS Files
```

**Data Processing:**
1. **Audio Data**: `OnAudio()` → `flvWriter.WriteAudio()` → FLV audio tag → FFmpeg
2. **Video Data**: `OnVideo()` → `flvWriter.WriteVideo()` → FLV video tag → FFmpeg  
3. **Metadata**: `OnSetDataFrame()` → `flvWriter.WriteScript()` → FLV script tag → FFmpeg

**FLV Tag Structure:**
- Tag Header (11 bytes): type, size, timestamp
- Tag Data (variable size): actual audio/video/metadata
- Previous Tag Size (4 bytes): size of previous tag

### 6. HLS Output Generation

FFmpeg generates HLS files in the output directory:

```
./streams/johndoe/
├── live.m3u8          # Master playlist
├── live_000.ts        # Video segments
├── live_001.ts
├── live_002.ts
└── ...
```

**HLS Playlist Structure:**
- `live.m3u8`: Contains references to `.ts` segments
- `live_XXX.ts`: Individual video segments (1 second each)
- Rolling window: keeps 3 segments, deletes old ones

### 7. HTTP Streaming

Viewers access streams via HTTP:

```
GET http://localhost:8080/stream/johndoe/live.m3u8
```

**HTTP Server Objects:**
```go
server := &Server{
    config:        config,
    streamManager: streamManager,
}
```

**Request Flow:**
1. Parse URL path: `/stream/johndoe/live.m3u8`
2. Extract username: `johndoe`
3. Check if stream directory exists: `./streams/johndoe/`
4. Serve file with appropriate headers (no-cache for live streams)

### 8. Stream Cleanup

When a client disconnects:

```go
// OnClose() is called
// Wait for reconnection delay (5 seconds)
time.Sleep(config.ReconnectDelay)

// Check if stream is still active
if streamProcess.IsActive() {
    // Stop FFmpeg process
    streamProcess.Stop(config)
    // Cleanup: remove from manager, set active=false
}
```

**Cleanup Process:**
1. Cancel FFmpeg context
2. Wait for FFmpeg to exit
3. Remove from stream manager
4. Set active state to false
5. Log cleanup completion

## Object Relationships

```
main.go
├── config.Config (global configuration)
├── stream.Manager (manages all streams)
├── http.Server (serves HLS)
└── rtmp.Server (accepts RTMP)
    └── rtmp.Handler (per connection)
        ├── auth.Authorizer (validates URLs)
        ├── models.ConnectionInfo (stores connection data)
        ├── stream.StreamProcess (FFmpeg process)
        └── flv.Writer (writes FLV tags)
```

## Authorization System

**Pattern Matching:**
- Patterns: `/live/{app}/{username}`
- TCURL: `rtmp://localhost/live/test/johndoe`
- Extracted: `{"app": "test", "username": "johndoe"}`

**Validation Rules:**
- TCURL must match an authorized pattern
- Extracted `username` must match `publishingName`
- Additional rules can be added in `ValidateAuthentication()`

## Thread Safety

- **Stream Manager**: Uses `sync.Map` for thread-safe stream storage
- **FLV Writer**: Uses `sync.Mutex` for thread-safe tag writing
- **Stream Process**: Uses `atomic.Bool` for thread-safe active state
- **Connection Info**: Uses `sync.RWMutex` for thread-safe access

## File Structure

```
.
├── cmd/main.go                 # Application entry point
├── internal/
│   ├── auth/
│   │   ├── authorizer.go       # Authorization logic
│   │   └── pattern.go          # Pattern matching utilities
│   ├── config/
│   │   └── config.go           # Configuration management
│   ├── flv/
│   │   ├── writer.go           # FLV tag writing
│   │   └── muxer.go            # FLV muxing utilities
│   ├── http/
│   │   └── server.go           # HTTP server for HLS
│   ├── models/
│   │   └── connection.go       # Data structures
│   ├── rtmp/
│   │   └── handler.go          # RTMP connection handling
│   └── stream/
│       ├── manager.go          # Stream lifecycle management
│       └── process.go          # Individual stream processes
└── streams/                    # HLS output directory
    └── {username}/
        ├── live.m3u8
        └── live_*.ts
```

## Usage Examples

**Publishing Streams:**
```bash
# Using OBS or any RTMP encoder
rtmp://localhost/live/test/johndoe
rtmp://localhost/live/myapp/alice
```

**Viewing Streams:**
```bash
# HLS playlist URLs
http://localhost:8080/stream/johndoe/live.m3u8
http://localhost:8080/stream/alice/live.m3u8
```

**Server Status:**
```bash
# View active streams
http://localhost:8080/
```

This architecture provides a robust, scalable solution for handling multiple concurrent RTMP streams with proper authorization, conversion to HLS, and HTTP delivery. 