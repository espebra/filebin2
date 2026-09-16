package web

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"hash"
	"io"
)

// contentDigest calculates the checksums of uploaded content. The content is
// fed through Writer while it is being written to disk, so that the checksums
// are available without reading the content back.
type contentDigest struct {
	md5    hash.Hash
	sha1   hash.Hash
	sha256 hash.Hash
}

func newContentDigest() *contentDigest {
	return &contentDigest{
		md5:    md5.New(),
		sha1:   sha1.New(),
		sha256: sha256.New(),
	}
}

// Writer returns a writer that feeds every hasher in the digest.
func (d *contentDigest) Writer() io.Writer {
	return io.MultiWriter(d.md5, d.sha1, d.sha256)
}

// MD5 returns the base64 encoded MD5 checksum, which is the encoding used by
// the Content-MD5 request header.
func (d *contentDigest) MD5() string {
	return base64.StdEncoding.EncodeToString(d.md5.Sum(nil))
}

// md5Hex converts a base64 encoded MD5 checksum, as stored, to the hex
// encoding used by md5sum(1). It returns an empty string if the input is not
// valid base64.
func md5Hex(b64 string) string {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return ""
	}
	return hex.EncodeToString(raw)
}

// SHA1 returns the hex encoded SHA1 checksum.
func (d *contentDigest) SHA1() string {
	return hex.EncodeToString(d.sha1.Sum(nil))
}

// SHA256 returns the hex encoded SHA256 checksum.
func (d *contentDigest) SHA256() string {
	return hex.EncodeToString(d.sha256.Sum(nil))
}
