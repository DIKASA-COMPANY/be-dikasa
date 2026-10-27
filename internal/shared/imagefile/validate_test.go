package imagefile

import (
	"bytes"
	"errors"
	"image"
	"image/png"
	"testing"
)

func TestBoundedUnchangedImageValidation(t *testing.T) {
	var b bytes.Buffer
	png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 2, 2)))
	raw := b.Bytes()
	v, e := Validate(Upload{Reader: bytes.NewReader(raw), Size: int64(len(raw)), Filename: "a.png", ContentType: "image/png"}, AvatarPolicy)
	if e != nil || !bytes.Equal(v.Bytes, raw) || len(v.SHA256) != 64 {
		t.Fatal("original bytes changed", e)
	}
	for _, u := range []Upload{{Reader: bytes.NewReader(raw), Size: int64(len(raw)), Filename: "a.png", ContentType: "image/jpeg"}, {Reader: bytes.NewReader(raw), Size: int64(len(raw)), Filename: "a.exe", ContentType: "image/png"}, {Reader: bytes.NewReader([]byte("bad")), Size: 3, Filename: "a.png", ContentType: "image/png"}} {
		if _, e := Validate(u, AvatarPolicy); !errors.Is(e, ErrInvalid) {
			t.Fatal("invalid accepted", e)
		}
	}
	if _, e := Validate(Upload{Reader: bytes.NewReader(raw), Size: 3145729}, AvatarPolicy); !errors.Is(e, ErrTooLarge) {
		t.Fatal("large file accepted", e)
	}
}
