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

// Compute reads an image from r, decodes it, and returns a 16-char hex pHash string.
// Returns empty string and nil error for unsupported/undecodable image formats.
// Returns empty string and error only for hashing failures on successfully decoded images.
func Compute(r io.Reader) (string, error) {
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
