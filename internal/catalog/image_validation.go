package catalog

import (
	"be-dikasa/internal/shared/imagefile"
	"errors"
)

// validateImage maps the shared image policy to catalog domain errors.
func validateImage(upload *ImageUpload) ([]byte, string, string, error) {
	if upload == nil {
		return nil, "", "", ErrInvalidImage
	}
	value, err := imagefile.Validate(imagefile.Upload{
		Reader: upload.Reader, Size: upload.Size, ContentType: upload.ContentType, Filename: upload.Filename,
	}, imagefile.StandardPolicy)
	if errors.Is(err, imagefile.ErrTooLarge) {
		return nil, "", "", ErrImageTooLarge
	}
	if err != nil {
		return nil, "", "", ErrInvalidImage
	}
	return value.Bytes, value.Extension, value.ContentType, nil
}
