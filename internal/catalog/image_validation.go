package catalog

import (
	"bytes"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
)

// validateImage checks bounded image content before either local or cloud persistence.
func validateImage(upload *ImageUpload) ([]byte, string, string, error) {
	if upload == nil || upload.Reader == nil {
		return nil, "", "", ErrInvalidImage
	}
	if upload.Size > maxImageBytes {
		return nil, "", "", ErrImageTooLarge
	}
	if upload.Size < 1 {
		return nil, "", "", ErrInvalidImage
	}
	content, err := io.ReadAll(io.LimitReader(upload.Reader, maxImageBytes+1))
	if err != nil {
		return nil, "", "", ErrInvalidImage
	}
	if int64(len(content)) > maxImageBytes {
		return nil, "", "", ErrImageTooLarge
	}
	declared, _, err := mime.ParseMediaType(upload.ContentType)
	if err != nil {
		return nil, "", "", ErrInvalidImage
	}
	detected := http.DetectContentType(content)
	extension := strings.ToLower(filepath.Ext(upload.Filename))
	allowedExtension := extension == ".png" || extension == ".jpg" || extension == ".jpeg"
	allowedMIME := declared == "image/png" || declared == "image/jpeg"
	if !allowedExtension || !allowedMIME || detected != declared {
		return nil, "", "", ErrInvalidImage
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(content))
	if err != nil || config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > 24_000_000 {
		return nil, "", "", ErrInvalidImage
	}
	_, decodedFormat, err := image.Decode(bytes.NewReader(content))
	if err != nil || (decodedFormat != "png" && decodedFormat != "jpeg") {
		return nil, "", "", ErrInvalidImage
	}
	if (decodedFormat == "png") != (declared == "image/png") {
		return nil, "", "", ErrInvalidImage
	}
	if decodedFormat == "jpeg" && declared != "image/jpeg" {
		return nil, "", "", ErrInvalidImage
	}
	if decodedFormat == "png" {
		extension = ".png"
	} else {
		extension = ".jpg"
	}
	return content, extension, declared, nil
}
