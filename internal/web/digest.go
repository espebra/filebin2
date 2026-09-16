package web

import (
	"crypto/md5"
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
	sha256 hash.Hash
}

func newContentDigest() *contentDigest {
	return &contentDigest{
		md5:    md5.New(),
		sha256: sha256.New(),
	}
}

// Writer returns a writer that feeds every hasher in the digest.
func (d *contentDigest) Writer() io.Writer {
	return io.MultiWriter(d.md5, d.sha256)
}

// MD5 returns the base64 encoded MD5 checksum, which is the encoding used by
// the Content-MD5 request header.
func (d *contentDigest) MD5() string {
	return base64.StdEncoding.EncodeToString(d.md5.Sum(nil))
}

// SHA256 returns the hex encoded SHA256 checksum.
func (d *contentDigest) SHA256() string {
	return hex.EncodeToString(d.sha256.Sum(nil))
}
