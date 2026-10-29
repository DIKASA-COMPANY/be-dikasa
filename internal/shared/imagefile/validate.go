// Package imagefile validates bounded original PNG/JPEG bytes for every image API.
package imagefile

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
)

// validate checks bounded image content before Supabase Storage persistence.
func validate(upload *Upload, policy Policy) ([]byte, string, string, error) {
	if upload == nil || upload.Reader == nil {
		return nil, "", "", ErrInvalid
	}
	if upload.Size > policy.MaxBytes {
		return nil, "", "", ErrTooLarge
	}
	if upload.Size < 1 {
		return nil, "", "", ErrInvalid
	}
	content, err := io.ReadAll(io.LimitReader(upload.Reader, policy.MaxBytes+1))
	if err != nil {
		return nil, "", "", ErrInvalid
	}
	if int64(len(content)) > policy.MaxBytes {
		return nil, "", "", ErrTooLarge
	}
	declared, _, err := mime.ParseMediaType(upload.ContentType)
	if err != nil {
		return nil, "", "", ErrInvalid
	}
	detected := http.DetectContentType(content)
	extension := strings.ToLower(filepath.Ext(upload.Filename))
	allowedExtension := extension == ".png" || extension == ".jpg" || extension == ".jpeg"
	allowedMIME := declared == "image/png" || declared == "image/jpeg"
	if !allowedExtension || !allowedMIME || detected != declared {
		return nil, "", "", ErrInvalid
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(content))
	if err != nil || config.Width <= 0 || config.Height <= 0 || config.Width > policy.MaxAxis || config.Height > policy.MaxAxis || int64(config.Width)*int64(config.Height) > policy.MaxPixels {
		return nil, "", "", ErrInvalid
	}
	_, decodedFormat, err := image.Decode(bytes.NewReader(content))
	if err != nil || (decodedFormat != "png" && decodedFormat != "jpeg") {
		return nil, "", "", ErrInvalid
	}
	if (decodedFormat == "png") != (declared == "image/png") {
		return nil, "", "", ErrInvalid
	}
	if decodedFormat == "jpeg" && declared != "image/jpeg" {
		return nil, "", "", ErrInvalid
	}
	if decodedFormat == "png" {
		extension = ".png"
	} else {
		extension = ".jpg"
	}
	return content, extension, declared, nil
}

// Upload contains untrusted file metadata and its bounded stream.
type Upload struct {
	Reader                io.Reader
	Size                  int64
	ContentType, Filename string
}

// Policy limits encoded and decoded input before allocating image pixels.
type Policy struct {
	MaxBytes  int64
	MaxAxis   int
	MaxPixels int64
}

// StandardPolicy is the single validation policy used by product, avatar, and cash-proof uploads.
var StandardPolicy = Policy{MaxBytes: 3 * 1024 * 1024, MaxAxis: 4096, MaxPixels: 16_000_000}

// ErrInvalid denotes corrupt or unsupported image content.
var ErrInvalid = errors.New("invalid image")

// ErrTooLarge denotes an encoded size beyond policy.
var ErrTooLarge = errors.New("image too large")

// Image holds unchanged validated bytes and safe metadata.
type Image struct {
	Bytes                          []byte
	Extension, ContentType, SHA256 string
}

// Validate checks bytes before persistence; no re-encoding alters evidence.
func Validate(u Upload, p Policy) (Image, error) {
	if p.MaxBytes < 1 || p.MaxAxis < 1 || p.MaxPixels < 1 {
		return Image{}, ErrInvalid
	}
	b, ext, mime, e := validate(&u, p)
	if e != nil {
		return Image{}, e
	}
	h := sha256.Sum256(b)
	return Image{b, ext, mime, hex.EncodeToString(h[:])}, nil
}
