package phash

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"strings"
	"testing"

	"golang.org/x/image/bmp"
	"golang.org/x/image/tiff"
)

// newTestImage creates a simple test image with a gradient pattern.
func newTestImage(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{
				R: uint8(x * 255 / w),
				G: uint8(y * 255 / h),
				B: 128,
				A: 255,
			})
		}
	}
	return img
}

// newSolidImage creates a uniform-color image.
func newSolidImage(w, h int, c color.Color) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	return img
}

func encodePNG(img image.Image) []byte {
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func encodeJPEG(img image.Image) []byte {
	var buf bytes.Buffer
	_ = jpeg.Encode(&buf, img, nil)
	return buf.Bytes()
}

func encodeGIF(img image.Image) []byte {
	var buf bytes.Buffer
	bounds := img.Bounds()
	palette := make(color.Palette, 256)
	for i := range palette {
		palette[i] = color.RGBA{R: uint8(i), G: uint8(i), B: uint8(i), A: 255}
	}
	palettedImg := image.NewPaletted(bounds, palette)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			palettedImg.Set(x, y, img.At(x, y))
		}
	}
	_ = gif.Encode(&buf, palettedImg, nil)
	return buf.Bytes()
}

func encodeBMP(img image.Image) []byte {
	var buf bytes.Buffer
	_ = bmp.Encode(&buf, img)
	return buf.Bytes()
}

func encodeTIFF(img image.Image) []byte {
	var buf bytes.Buffer
	_ = tiff.Encode(&buf, img, nil)
	return buf.Bytes()
}

func TestComputePNG(t *testing.T) {
	img := newTestImage(64, 64)
	data := encodePNG(img)
	hash, err := Compute(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hash) != 16 {
		t.Fatalf("expected 16-char hex string, got %q (len %d)", hash, len(hash))
	}
}

func TestComputeJPEG(t *testing.T) {
	img := newTestImage(64, 64)
	data := encodeJPEG(img)
	hash, err := Compute(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hash) != 16 {
		t.Fatalf("expected 16-char hex string, got %q (len %d)", hash, len(hash))
	}
}

func TestComputeGIF(t *testing.T) {
	img := newTestImage(64, 64)
	data := encodeGIF(img)
	hash, err := Compute(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hash) != 16 {
		t.Fatalf("expected 16-char hex string, got %q (len %d)", hash, len(hash))
	}
}

func TestComputeBMP(t *testing.T) {
	img := newTestImage(64, 64)
	data := encodeBMP(img)
	hash, err := Compute(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hash) != 16 {
		t.Fatalf("expected 16-char hex string, got %q (len %d)", hash, len(hash))
	}
}

func TestComputeTIFF(t *testing.T) {
	img := newTestImage(64, 64)
	data := encodeTIFF(img)
	hash, err := Compute(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hash) != 16 {
		t.Fatalf("expected 16-char hex string, got %q (len %d)", hash, len(hash))
	}
}

func TestComputeUnsupportedFormat(t *testing.T) {
	data := []byte("this is not an image")
	hash, err := Compute(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("expected nil error for unsupported format, got: %v", err)
	}
	if hash != "" {
		t.Fatalf("expected empty string for unsupported format, got %q", hash)
	}
}

func TestComputeEmptyReader(t *testing.T) {
	hash, err := Compute(bytes.NewReader(nil))
	if err != nil {
		t.Fatalf("expected nil error for empty reader, got: %v", err)
	}
	if hash != "" {
		t.Fatalf("expected empty string for empty reader, got %q", hash)
	}
}

func TestComputeHexFormat(t *testing.T) {
	img := newTestImage(64, 64)
	data := encodePNG(img)
	hash, err := Compute(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, c := range hash {
		if !strings.ContainsRune("0123456789abcdef", c) {
			t.Fatalf("hash contains non-hex character %q: %s", c, hash)
		}
	}
}

func TestComputeDeterministic(t *testing.T) {
	img := newTestImage(64, 64)
	data := encodePNG(img)
	hash1, err := Compute(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	hash2, err := Compute(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hash1 != hash2 {
		t.Fatalf("expected identical hashes for same input, got %q and %q", hash1, hash2)
	}
}

func TestComputeSameImageDifferentFormats(t *testing.T) {
	img := newTestImage(64, 64)

	pngHash, _ := Compute(bytes.NewReader(encodePNG(img)))
	bmpHash, _ := Compute(bytes.NewReader(encodeBMP(img)))
	tiffHash, _ := Compute(bytes.NewReader(encodeTIFF(img)))

	// Lossless formats encoding the same image should produce the same pHash.
	if pngHash != bmpHash {
		t.Errorf("PNG and BMP hashes differ: %s vs %s", pngHash, bmpHash)
	}
	if pngHash != tiffHash {
		t.Errorf("PNG and TIFF hashes differ: %s vs %s", pngHash, tiffHash)
	}
}

func TestComputeDifferentImages(t *testing.T) {
	white := newSolidImage(64, 64, color.White)
	black := newSolidImage(64, 64, color.Black)

	whiteHash, _ := Compute(bytes.NewReader(encodePNG(white)))
	blackHash, _ := Compute(bytes.NewReader(encodePNG(black)))

	if whiteHash == blackHash {
		t.Errorf("expected different hashes for white and black images, both got %s", whiteHash)
	}
}
