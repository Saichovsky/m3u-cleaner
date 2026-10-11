# m3u-cleaner

A Go-based IPTV playlist cleaner designed to run as an init container in Kubernetes. It fetches IPTV channel playlists from configurable URLs, filters out low-resolution channels, validates stream availability, and generates a clean M3U playlist for use with IPTV web services.

## Features

- Fetches channel playlists from configurable URLs (not hard-coded to a single source)
- Filters out low-resolution channels (explicit resolutions below configurable height, default 720p)
- Excludes channels by configurable keywords (case-insensitive substring match, e.g. `bbc pashto`)
- Validates stream availability using concurrent HTTP requests
- Generates a clean `index.m3u` playlist file
- Designed to run quickly as a Kubernetes init container
- Minimal Docker image using multi-stage build (scratch base)
- Configurable via a mandatory TOML config file

## How It Works

1. **Fetch**: Downloads M3U playlists from URLs listed in the config file
2. **Filter**: Removes channels with an explicit resolution below the configured height (e.g. "(360p)", "(480p)"). Channels without a resolution marker are kept, since most do not declare one. Adult content is not included in most feeds, so no separate adult filter is needed.
3. **Exclude**: Drops any channel whose EXTINF metadata matches an exclude keyword
4. **Validate**: Concurrently checks if stream URLs are accessible (HTTP 200)
5. **Output**: Writes valid channels to `index.m3u` in the specified output directory

The output file is rewritten **in place** (truncate, never temp-file + rename) so the existing inode of `index.m3u` is preserved for the pod that serves it.

## Project Structure

```
cmd/m3u-cleaner/       CLI entrypoint: flags/env parsing and pipeline orchestration
internal/config/       TOML config loading, defaults, and validation
internal/iptv/         IPTV domain: M3U parsing, filtering, stream validation
internal/playlist/     M3U playlist writer (in-place, inode-preserving)
config.toml            Example config file (mandatory)
```

## Usage

### As a Standalone Application

A config file is **required**. Pass it via `--config` flag or `CONFIG` env var.

```bash
# Use a TOML config file
go run ./cmd/m3u-cleaner --config=config.toml

# Or via environment variable
CONFIG=config.toml go run ./cmd/m3u-cleaner

# Override output directory
go run ./cmd/m3u-cleaner --config=config.toml --outdir=/path/to/output
```

### As a Docker Container

A sample config is baked into the image at `/etc/iptv/config.toml`, with `CONFIG`
pointing to it by default.

```bash
# Build the image
docker build -t m3u-cleaner .

# Run with the baked-in sample config
docker run --rm \
  -e LISTDIR=/output \
  -v $(pwd)/output:/output \
  m3u-cleaner

# Run with a custom config mounted over the sample
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

A TOML config file is **mandatory**. The application will not start without one.

### TOML Config File

See `config.toml` for the full example.

```toml
# REQUIRED: Playlist URLs to fetch and process
playlist_urls = [
  "https://iptv-org.github.io/iptv/countries/ke.m3u",
  "https://iptv-org.github.io/iptv/countries/uk.m3u",
  "https://iptv-org.github.io/iptv/countries/us.m3u",
]

# OPTIONAL: Channels to exclude (case-insensitive substring match)
exclude_channels = ["bbc pashto", "cbeebies"]

# OPTIONAL: Minimum resolution height (default: 720)
min_resolution_height = 720

# OPTIONAL: Per-request timeout for stream validation (default: 10)
timeout_seconds = 10

# OPTIONAL: Number of concurrent stream checks (default: 50)
concurrency = 50
```

| Field | Required | Default | Description |
|---|---|---|---|
| `playlist_urls` | **Yes** | — | List of M3U playlist URLs to fetch and process |
| `exclude_channels` | No | none | Substrings (case-insensitive) matched against EXTINF metadata; matches are dropped |
| `min_resolution_height` | No | 720 | Channels with an explicit resolution below this are removed |
| `timeout_seconds` | No | 10 | Per-request timeout for stream URL validation |
| `concurrency` | No | 50 | Number of parallel stream URL checks |

### Validation

The config is validated at startup. The following rules are enforced:

- `playlist_urls` must contain at least one URL
- `min_resolution_height` must be positive (> 0)
- `timeout_seconds` must be positive (> 0)
- `concurrency` must be positive (> 0)

Invalid configs cause the application to exit immediately with a non-zero exit code and a descriptive error message.

### Environment Variables

- `CONFIG`: Path to a TOML config file (required — no default)
- `LISTDIR`: Output directory for index.m3u (default: `/usr/share/nginx/html`)

### Command-Line Flags

- `--config`: Override `CONFIG` environment variable
- `--outdir`: Override `LISTDIR` environment variable

Configuration precedence: Flag → Environment Variable

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
