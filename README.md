# m3u-cleaner

A Go-based IPTV playlist cleaner designed to run as an init container in Kubernetes. It fetches IPTV channel playlists from iptv-org, filters out low-resolution channels, validates stream availability, and generates a clean M3U playlist for use with IPTV web services.

## Features

- Fetches channel playlists from [iptv-org](https://github.com/iptv-org/iptv) for specified countries
- Filters out low-resolution channels (explicit resolutions below configurable height, default 720p)
- Excludes channels by configurable keywords (case-insensitive substring match, e.g. `bbc pashto`)
- Validates stream availability using concurrent HTTP requests
- Generates a clean `index.m3u` playlist file
- Designed to run quickly as a Kubernetes init container
- Minimal Docker image using multi-stage build (scratch base)
- Configurable via environment variables, command-line flags, or a TOML config file

## How It Works

1. **Fetch**: Downloads M3U playlists for specified countries from iptv-org
2. **Filter**: Removes channels with an explicit resolution below the configured height (e.g. "(360p)", "(480p)"). Channels without a resolution marker are kept, since most do not declare one. Adult content is not included in iptv-org country feeds, so no separate adult filter is needed.
3. **Exclude**: Drops any channel whose EXTINF metadata matches an exclude keyword
4. **Validate**: Concurrently checks if stream URLs are accessible (HTTP 200)
5. **Output**: Writes valid channels to `index.m3u` in the specified output directory

The output file is rewritten **in place** (truncate, never temp-file + rename) so the existing inode of `index.m3u` is preserved for the pod that serves it.

## Project Structure

```
cmd/m3u-cleaner/       CLI entrypoint: flags/env parsing and pipeline orchestration
internal/config/       TOML config loading and defaults
internal/iptv/         IPTV domain: M3U parsing, filtering, stream validation
internal/playlist/     M3U playlist writer (in-place, inode-preserving)
config.toml            Example config file
```

## Usage

### As a Standalone Application

```bash
# Basic usage with defaults
go run ./cmd/m3u-cleaner

# Specify countries via flag
go run ./cmd/m3u-cleaner --countries=ke,uk,us,fr

# Specify output directory via flag
go run ./cmd/m3u-cleaner --outdir=/path/to/output

# Use a TOML config file
go run ./cmd/m3u-cleaner --config=config.toml

# Using environment variables
COUNTRIES=ke,uk,us LISTDIR=/app/data CONFIG=config.toml go run ./cmd/m3u-cleaner
```

### As a Docker Container

```bash
# Build the image
docker build -t m3u-cleaner .

# Run with defaults
docker run --rm \
  -e LISTDIR=/output \
  -v $(pwd)/output:/output \
  m3u-cleaner

# Run with custom countries
docker run --rm \
  -e COUNTRIES=ke,uk,us,fr,de \
  -e LISTDIR=/output \
  -v $(pwd)/output:/output \
  m3u-cleaner

# Run with a TOML config file mounted over a baked-in sample
# (also available at /etc/iptv/config.toml in the image)
docker run --rm \
  -e LISTDIR=/output \
  -e CONFIG=/etc/iptv/config.toml \
  -v $(pwd)/output:/output \
  -v $(pwd)/config.toml:/etc/iptv/config.toml:ro \
  m3u-cleaner

# Alternatively pass the config path as a CLI flag
docker run --rm \
  -e LISTDIR=/output \
  -v $(pwd)/output:/output \
  -v $(pwd)/config.toml:/etc/iptv/config.toml:ro \
  m3u-cleaner --config=/etc/iptv/config.toml
```

### As a Kubernetes Init Container

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: iptv-service
spec:
  initContainers:
  - name: iptv-playlist-cleaner
    image: m3u-cleaner:latest
    env:
    - name: COUNTRIES
      value: "ke,uk,us"
    - name: LISTDIR
      value: "/usr/share/nginx/html"
    - name: CONFIG
      value: "/etc/iptv/config.toml"
    volumeMounts:
    - name: playlist-volume
      mountPath: /usr/share/nginx/html
    - name: config-volume
      mountPath: /etc/iptv
      readOnly: true
  containers:
  - name: iptv-web-service
    image: nginx:alpine
    volumeMounts:
    - name: playlist-volume
      mountPath: /usr/share/nginx/html
  volumes:
  - name: playlist-volume
    emptyDir: {}
  - name: config-volume
    configMap:
      name: iptv-config
```

## Configuration

The application can be configured via:

### TOML Config File

See `config.toml` for the full example. Every field is optional.

```toml
exclude_channels = ["bbc pashto", "cbeebies"]
min_resolution_height = 720
timeout_seconds = 10
concurrency = 50
```

- `exclude_channels`: substrings (case-insensitive) matched against channel EXTINF metadata; any match drops the channel
- `min_resolution_height`: channels explicitly below this are removed (default: 720)
- `timeout_seconds`: per-request timeout for stream validation (default: 10)
- `concurrency`: number of parallel stream checks (default: 50)

### Environment Variables

- `COUNTRIES`: Comma-separated list of country codes (default: "ke,uk,us")
- `LISTDIR`: Output directory for index.m3u (default: "/usr/share/nginx/html")
- `CONFIG`: Path to a TOML config file (default: none, uses built-in defaults; the Docker image provides a sample at `/etc/iptv/config.toml`)

### Command-Line Flags

- `--countries`: Override COUNTRIES environment variable
- `--outdir`: Override LISTDIR environment variable
- `--config`: Override CONFIG environment variable

Configuration precedence: Flag → Environment Variable → Config file → Default

## Performance

- Uses concurrent stream validation (50 workers by default, configurable)
- 10-second timeout per stream validation (configurable); 15 seconds for playlist fetches
- Early cancellation: closes HTTP connection immediately after receiving HTTP 200 status to save bandwidth
- Efficient streaming M3U parsing without loading entire files into memory
- Low-resolution and exclude filters run in-memory before validation to avoid needless HTTP requests
- All timeout values are passed as parameters to functions with sane defaults, making them easy to test

## Output

Generates a standard M3U playlist file named `index.m3u` in the specified output directory:
```
#EXTM3U
#EXTINF:-1 tvg-id="bbc.one.uk" group-title="General" ,BBC One
http://example.com/bbc1.m3u8
#EXTINF:-1 tvg-id="channel4.uk" group-title="General" ,Channel 4
http://example.com/channel4.m3u8
```

## Building

```bash
# Build Docker image
docker build -t m3u-cleaner .

# Or build the static binary directly
CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o m3u-cleaner ./cmd/m3u-cleaner
```

## Testing

```bash
go test ./...
```