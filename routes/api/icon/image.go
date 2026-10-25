package icon

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"mime"
	"net/http"

	_ "image/gif" // Register the supported raster decoders.
	_ "image/jpeg"
)

const (
	maxIconBytes     = 1 << 20
	maxIconPixels    = 1024 * 1024
	maxIconDimension = 2048
)

var errInvalidImage = errors.New("icon must be a small PNG, JPEG or GIF image")

func readIcon(response *http.Response) (*Icon, error) {
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, errInvalidImage
	}

	mediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil ||
		(mediaType != "image/png" && mediaType != "image/jpeg" && mediaType != "image/gif") {
		return nil, errInvalidImage
	}

	if response.ContentLength > maxIconBytes {
		return nil, errInvalidImage
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, maxIconBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read icon: %w", err)
	}

	if len(body) > maxIconBytes {
		return nil, errInvalidImage
	}

	if !validImageConfig(body, mediaType) {
		return nil, errInvalidImage
	}

	decoded, _, err := image.Decode(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("decode icon: %w", err)
	}

	// Re-encoding drops trailing payloads and metadata, including active content.
	var output cappedBuffer

	err = png.Encode(&output, decoded)
	if err != nil {
		return nil, fmt.Errorf("encode icon: %w", err)
	}

	return &Icon{Data: output.Bytes()}, nil
}

// Inspect dimensions before allocating pixels, including for highly compressed images.
func validImageConfig(body []byte, mediaType string) bool {
	config, format, err := image.DecodeConfig(bytes.NewReader(body))
	if err != nil || mediaType != "image/"+format {
		return false
	}

	return config.Width > 0 && config.Height > 0 &&
		config.Width <= maxIconDimension && config.Height <= maxIconDimension &&
		config.Width <= maxIconPixels/config.Height
}

type cappedBuffer struct {
	bytes.Buffer
}

func (buffer *cappedBuffer) Write(data []byte) (int, error) {
	if len(data) > maxIconBytes-buffer.Len() {
		return 0, errInvalidImage
	}

	written, err := buffer.Buffer.Write(data)
	if err != nil {
		return written, fmt.Errorf("buffer icon: %w", err)
	}

	return written, nil
}
