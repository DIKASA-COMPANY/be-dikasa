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
	v, e := Validate(Upload{Reader: bytes.NewReader(raw), Size: int64(len(raw)), Filename: "a.png", ContentType: "image/png"}, StandardPolicy)
	if e != nil || !bytes.Equal(v.Bytes, raw) || len(v.SHA256) != 64 {
		t.Fatal("original bytes changed", e)
	}
	for _, u := range []Upload{{Reader: bytes.NewReader(raw), Size: int64(len(raw)), Filename: "a.png", ContentType: "image/jpeg"}, {Reader: bytes.NewReader(raw), Size: int64(len(raw)), Filename: "a.exe", ContentType: "image/png"}, {Reader: bytes.NewReader([]byte("bad")), Size: 3, Filename: "a.png", ContentType: "image/png"}} {
		if _, e := Validate(u, StandardPolicy); !errors.Is(e, ErrInvalid) {
			t.Fatal("invalid accepted", e)
		}
	}
	if _, e := Validate(Upload{Reader: bytes.NewReader(raw), Size: 3145729}, StandardPolicy); !errors.Is(e, ErrTooLarge) {
		t.Fatal("large file accepted", e)
	}
}

func TestStandardPolicyIsSharedByEveryImageAPI(t *testing.T) {
	if StandardPolicy.MaxBytes != 3*1024*1024 || StandardPolicy.MaxAxis != 4096 || StandardPolicy.MaxPixels != 16_000_000 {
		t.Fatalf("unexpected standard image policy: %+v", StandardPolicy)
	}
}

func TestValidateIntentAcceptsOnlyBoundedPNGOrJPEGHints(t *testing.T) {
	for contentType, extension := range map[string]string{"image/png": ".png", "image/jpeg": ".jpg"} {
		actual, err := ValidateIntent(contentType, 1024, StandardPolicy)
		if err != nil || actual != extension {
			t.Fatalf("intent %q: extension=%q error=%v", contentType, actual, err)
		}
	}
	for _, value := range []struct {
		contentType string
		size        int64
		want        error
	}{{"image/gif", 1024, ErrInvalid}, {"image/png", 0, ErrInvalid}, {"image/jpeg", StandardPolicy.MaxBytes + 1, ErrTooLarge}} {
		if _, err := ValidateIntent(value.contentType, value.size, StandardPolicy); !errors.Is(err, value.want) {
			t.Fatalf("intent type=%q size=%d: got %v want %v", value.contentType, value.size, err, value.want)
		}
	}
}
