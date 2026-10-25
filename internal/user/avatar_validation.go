package user

import (
	"be-dikasa/internal/shared/imagefile"
	"errors"
)

// validateAvatar retains the user-facing errors while sharing bounded byte validation.
func validateAvatar(u *AvatarUpload) ([]byte, string, string, error) {
	if u == nil {
		return nil, "", "", ErrInvalidAvatar
	}
	v, e := imagefile.Validate(imagefile.Upload{Reader: u.Reader, Size: u.Size, ContentType: u.ContentType, Filename: u.Filename}, imagefile.AvatarPolicy)
	if errors.Is(e, imagefile.ErrTooLarge) {
		return nil, "", "", ErrAvatarTooLarge
	}
	if e != nil {
		return nil, "", "", ErrInvalidAvatar
	}
	return v.Bytes, v.Extension, v.ContentType, nil
}
