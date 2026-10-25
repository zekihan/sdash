package icon

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"strings"
	"testing"
)

func pngFixture(t *testing.T) []byte {
	t.Helper()

	picture := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	picture.Set(0, 0, color.NRGBA{R: 255, G: 0, B: 0, A: 255})

	var encoded bytes.Buffer

	err := png.Encode(&encoded, picture)
	if err != nil {
		t.Fatal(err)
	}

	return encoded.Bytes()
}

func imageResponse(body []byte, contentType string) *http.Response {
	response := new(http.Response)
	response.StatusCode = http.StatusOK
	response.Header = http.Header{"Content-Type": {contentType}}
	response.Body = io.NopCloser(bytes.NewReader(body))
	response.ContentLength = int64(len(body))

	return response
}

func TestReadIconReencodesRasterImages(t *testing.T) {
	picture := image.NewRGBA(image.Rect(0, 0, 2, 2))

	var jpegData, gifData bytes.Buffer

	err := jpeg.Encode(&jpegData, picture, nil)
	if err != nil {
		t.Fatal(err)
	}

	err = gif.Encode(&gifData, picture, nil)
	if err != nil {
		t.Fatal(err)
	}

	for _, fixture := range []struct {
		name string
		mime string
		body []byte
	}{
		{
			name: "PNG with trailing HTML",
			body: append(pngFixture(t), []byte("<script>alert(1)</script>")...),
			mime: "image/png",
		},
		{name: "JPEG", body: jpegData.Bytes(), mime: "image/jpeg"},
		{name: "GIF", body: gifData.Bytes(), mime: "image/gif"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			response := imageResponse(fixture.body, fixture.mime)
			defer response.Body.Close()

			icon, err := readIcon(response)
			if err != nil {
				t.Fatal(err)
			}

			decoded, err := png.Decode(bytes.NewReader(icon.Data))
			if err != nil || decoded.Bounds() != picture.Bounds() ||
				bytes.Contains(icon.Data, []byte("<script>")) {
				t.Fatalf("invalid canonical PNG: %v", err)
			}
		})
	}
}

func TestReadIconRejectsActiveAndInvalidContent(t *testing.T) {
	for _, fixture := range []struct {
		name string
		mime string
		body []byte
	}{
		{name: "HTML", body: []byte("<script>alert(1)</script>"), mime: "text/html"},
		{
			name: "SVG",
			body: []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`),
			mime: "image/svg+xml",
		},
		{name: "HTML as PNG", body: []byte("<html>bad</html>"), mime: "image/png"},
		{name: "malformed PNG", body: []byte("fake-icon-data"), mime: "image/png"},
		{name: "PNG as HTML", body: pngFixture(t), mime: "text/html"},
		{name: "mismatched raster MIME", body: pngFixture(t), mime: "image/jpeg"},
		{name: "missing MIME", body: pngFixture(t), mime: ""},
		{name: "truncated PNG", body: pngFixture(t)[:40], mime: "image/png"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			response := imageResponse(fixture.body, fixture.mime)
			defer response.Body.Close()

			_, err := readIcon(response)
			if err == nil {
				t.Fatal("accepted invalid/active image")
			}
		})
	}

	response := imageResponse(pngFixture(t), "image/png")
	defer response.Body.Close()

	response.StatusCode = http.StatusNotFound

	_, err := readIcon(response)
	if err == nil {
		t.Fatal("accepted error response")
	}
}

type countingReader struct {
	reader io.Reader
	read   int
}

func (reader *countingReader) Read(data []byte) (int, error) {
	count, err := reader.reader.Read(data)
	reader.read += count

	if err != nil {
		return count, fmt.Errorf("fixture read: %w", err)
	}

	return count, nil
}

func TestReadIconByteLimits(t *testing.T) {
	for _, length := range []int64{maxIconBytes + 1, -1, maxIconBytes} {
		reader := &countingReader{
			reader: strings.NewReader(strings.Repeat("x", maxIconBytes+100)),
			read:   0,
		}

		response := imageResponse(nil, "image/png")

		response.ContentLength = length

		response.Body = io.NopCloser(reader)

		_, err := readIcon(response)

		closeErr := response.Body.Close()
		if closeErr != nil {
			t.Fatal(closeErr)
		}

		if err == nil {
			t.Fatal("accepted oversized response")
		}

		if reader.read > maxIconBytes+1 || (length > maxIconBytes && reader.read != 0) {
			t.Fatalf("unbounded read: declared %d, read %d", length, reader.read)
		}
	}

	// Exactly at the cap is allowed; re-encoding removes the padding.
	body := make([]byte, maxIconBytes)
	copy(body, pngFixture(t))

	response := imageResponse(body, "image/png")
	defer response.Body.Close()

	_, err := readIcon(response)
	if err != nil {
		t.Fatalf("rejected image at byte cap: %v", err)
	}
}

func TestReadIconPixelLimits(t *testing.T) {
	for _, bounds := range []image.Rectangle{
		image.Rect(0, 0, maxIconDimension+1, 1),
		image.Rect(0, 0, 1025, 1025),
	} {
		var encoded bytes.Buffer

		err := png.Encode(&encoded, image.NewGray(bounds))
		if err != nil {
			t.Fatal(err)
		}

		response := imageResponse(encoded.Bytes(), "image/png")

		_, err = readIcon(response)

		closeErr := response.Body.Close()
		if closeErr != nil {
			t.Fatal(closeErr)
		}

		if err == nil {
			t.Fatalf("accepted excessive dimensions %v", bounds)
		}
	}
}

func TestEncodedOutputLimit(t *testing.T) {
	var output cappedBuffer

	_, err := output.Write(make([]byte, maxIconBytes))
	if err != nil {
		t.Fatal(err)
	}

	_, err = output.Write([]byte{1})
	if err == nil || output.Len() != maxIconBytes {
		t.Fatal("encoded output exceeded cap")
	}
}
