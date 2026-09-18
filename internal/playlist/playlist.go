package playlist

import (
	"bufio"
	"os"
	"path/filepath"

	"m3u-cleaner/internal/iptv"
)

// OutputFile is the name of the generated playlist.
const OutputFile = "index.m3u"

// WritePlaylist writes the channels to outputPath as a standard M3U file,
// creating parent directories as needed. The existing file is truncated in
// place (never temp-file + rename) so the inode is preserved for readers.
func WritePlaylist(outputPath string, channels []iptv.Channel) (err error) {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return err
	}

	file, err := os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := file.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	writer := bufio.NewWriterSize(file, 64*1024)
	if _, err := writer.WriteString("#EXTM3U\n"); err != nil {
		return err
	}
	for _, ch := range channels {
		if _, err := writer.WriteString(ch.Metadata); err != nil {
			return err
		}
		if err := writer.WriteByte('\n'); err != nil {
			return err
		}
		if _, err := writer.WriteString(ch.URL); err != nil {
			return err
		}
		if err := writer.WriteByte('\n'); err != nil {
			return err
		}
	}
	return writer.Flush()
}
