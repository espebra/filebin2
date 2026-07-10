package phash

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
	"io"

	"github.com/corona10/goimagehash"
)

// maxPixels caps the number of pixels we are willing to decode. The standard
// library image decoders allocate the destination pixel buffer based on the
// dimensions declared in the file header before reading pixel data, so a tiny
// crafted file can declare enormous dimensions and exhaust memory (a
// "decompression"/pixel bomb). 50 megapixels is generous for legitimate
// images while keeping the worst-case allocation bounded.
const maxPixels = 50 * 1000 * 1000

// Compute reads an image from r, decodes it, and returns a 16-char hex pHash string.
// Returns empty string and nil error for unsupported/undecodable/oversized image formats.
// Returns empty string and error only for hashing failures on successfully decoded images.
//
// r must be seekable: the image header is inspected first (via DecodeConfig) to
// reject oversized images before the full decode allocates pixel memory, then r
// is rewound for the full decode.
func Compute(r io.ReadSeeker) (string, error) {
	// Inspect the header only (cheap) to reject pixel bombs before decoding.
	cfg, _, err := image.DecodeConfig(r)
	if err != nil {
		return "", nil // unsupported/undecodable format, not an error
	}
	// Guard against negative/overflowing dimensions and excessive pixel counts.
	if cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > maxPixels {
		return "", nil // refuse to decode oversized images
	}
	// Rewind so the full decode sees the complete stream.
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("seek: %w", err)
	}

	img, _, err := image.Decode(r)
	if err != nil {
		return "", nil // unsupported format, not an error
	}
	hash, err := goimagehash.PerceptionHash(img)
	if err != nil {
		return "", fmt.Errorf("perception hash: %w", err)
	}
	return fmt.Sprintf("%016x", hash.GetHash()), nil
}
