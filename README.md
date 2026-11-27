# m3u-cleaner

A Go-based IPTV playlist cleaner designed to run as an init container in Kubernetes. It fetches TV and radio playlists from configurable URLs, filters low-resolution TV channels, validates stream availability, and generates a single clean M3U playlist for use with IPTV web services.

## Features

- Fetches TV and radio playlists from independently configured sections
- Filters out low-resolution TV channels only (explicit resolutions below configurable height, default 720p)
- Tags radio entries with `radio="true"` for players that distinguish audio stations
- Excludes channels per section by configurable keywords (case-insensitive substring match, e.g. `bbc pashto`)
- Validates stream availability using per-section timeout and concurrency settings
- Generates a single clean `index.m3u` playlist file
- Skips all work when `index.m3u` already exists, unless `--force` is passed
- Designed to run quickly as a Kubernetes init container
- Minimal Docker image using multi-stage build (scratch base)
- Configurable via a mandatory TOML config file

## How It Works

1. **Fetch**: Downloads M3U playlists from the `[tv]` and `[radio]` sections of the config file
2. **Tag**: Adds `radio="true"` to radio entries that do not already declare it
3. **Filter**: For TV, removes channels with an explicit resolution below the configured height (e.g. "(360p)", "(480p)"); channels without a resolution marker are kept. Radio is not resolution-filtered. Each section then applies its own exclude keywords. Adult content is not included in most feeds, so no separate adult filter is needed.
4. **Validate**: Concurrently checks if stream URLs are accessible (HTTP 200), using each section's timeout and concurrency
5. **Output**: Writes valid TV channels followed by valid radio channels to a single `index.m3u` in the specified output directory

If `index.m3u` already exists in the output directory, the application logs that
it is skipping and exits successfully (code 0) **before** fetching anything.
Pass `--force` to regenerate the playlist.

The output file is rewritten **in place** (truncate, never temp-file + rename) so the existing inode of `index.m3u` is preserved for the pod that serves it.

## Project Structure

```
cmd/m3u-cleaner/       CLI entrypoint: flags/env parsing and pipeline orchestration
internal/config/       TOML config loading, defaults, and validation
internal/iptv/         IPTV domain: M3U parsing, filtering, stream validation
internal/playlist/     Playlist existence check and writer (in-place, inode-preserving)
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

# Regenerate even when index.m3u already exists
go run ./cmd/m3u-cleaner --config=config.toml --force
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

If `/output/index.m3u` already exists, the container skips regeneration and
exits successfully. Add `--force` to regenerate it.

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
    # Regenerate index.m3u on every pod start. Remove --force to reuse an
    # existing playlist on a persistent volume.
    args: ["--force"]
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
# TV section: resolution filtering and TV-specific validation settings
[tv]
playlist_urls = [
  "https://iptv-org.github.io/iptv/countries/ke.m3u",
  "https://iptv-org.github.io/iptv/countries/uk.m3u",
  "https://iptv-org.github.io/iptv/countries/us.m3u",
]
exclude_channels = ["bbc pashto", "cbeebies"]
min_resolution_height = 720
timeout_seconds = 10
concurrency = 50

# Radio section: entries are tagged radio="true" in the output
[radio]
playlist_urls = [
  "https://raw.githubusercontent.com/AlonDrilich/radio-playlists/refs/heads/main/countries/ke.m3u",
]
exclude_channels = ["ghett", "CARRIBEAN"]
timeout_seconds = 15
concurrency = 20
```

| Field | Required | Default | Description |
|---|---|---|---|
| `[tv].playlist_urls` | Yes* | — | List of TV M3U playlist URLs to fetch and process |
| `[tv].exclude_channels` | No | none | Substrings (case-insensitive) matched against TV EXTINF metadata; matches are dropped |
| `[tv].min_resolution_height` | No | 720 | TV channels with an explicit resolution below this are removed |
| `[tv].timeout_seconds` | No | 10 | Per-request timeout for TV stream URL validation |
| `[tv].concurrency` | No | 50 | Number of parallel TV stream URL checks |
| `[radio].playlist_urls` | Yes* | — | List of radio M3U playlist URLs to fetch and process |
| `[radio].exclude_channels` | No | none | Substrings (case-insensitive) matched against radio EXTINF metadata; matches are dropped |
| `[radio].timeout_seconds` | No | 15 | Per-request timeout for radio stream URL validation |
| `[radio].concurrency` | No | 20 | Number of parallel radio stream URL checks |

\* At least one of `[tv].playlist_urls` or `[radio].playlist_urls` must contain a URL. A TV-only or radio-only config is valid. `min_resolution_height` applies only to TV and is not used under `[radio]`.

### Validation

The config is validated at startup. The following rules are enforced:

- At least one of `tv.playlist_urls` or `radio.playlist_urls` must contain a URL
- If `[tv]` has URLs: `min_resolution_height`, `timeout_seconds`, and `concurrency` must be positive (> 0)
- If `[radio]` has URLs: `timeout_seconds` and `concurrency` must be positive (> 0)

Invalid configs cause the application to exit immediately with a non-zero exit code and a descriptive error message.

### Environment Variables

- `CONFIG`: Path to a TOML config file (required — no default)
- `LISTDIR`: Output directory for index.m3u (default: `/usr/share/nginx/html`)

### Command-Line Flags

- `--config`: Override `CONFIG` environment variable
- `--outdir`: Override `LISTDIR` environment variable
- `--force`: Regenerate `index.m3u` even if it already exists

Configuration precedence: Flag → Environment Variable

## Performance

- Uses concurrent stream validation with per-section settings (TV: 50 workers/10s, radio: 20 workers/15s by default)
- 15 seconds for playlist fetches
- Early cancellation: closes HTTP connection immediately after receiving HTTP 200 status to save bandwidth
- Efficient streaming M3U parsing without loading entire files into memory
- Low-resolution and exclude filters run in-memory before validation to avoid needless HTTP requests
- All timeout values are passed as parameters to functions with sane defaults, making them easy to test

## Output

Generates a single standard M3U playlist file named `index.m3u` in the specified output directory. TV channels are written first, followed by radio channels; radio entries carry `radio="true"`:
```
#EXTM3U
#EXTINF:-1 tvg-id="bbc.one.uk" group-title="General" ,BBC One
http://example.com/bbc1.m3u8
#EXTINF:-1 tvg-id="channel4.uk" group-title="General" ,Channel 4
http://example.com/channel4.m3u8
#EXTINF:-1 group-title="Kenya" radio="true",KBC
http://stream.example.com/kbc
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
