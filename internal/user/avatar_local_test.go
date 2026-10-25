package user

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLocalAvatarRoundTripAndConfinement(t *testing.T) {
	s, e := NewLocalAvatarStore(t.TempDir(), "http://localhost:8081")
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	var b bytes.Buffer
	png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 2, 2)))
	raw := b.Bytes()
	a, e := s.Save(context.Background(), "11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222", &AvatarUpload{Reader: bytes.NewReader(raw), Size: int64(len(raw)), ContentType: "image/png", Filename: "photo.png"})
	if e != nil {
		t.Fatal(e)
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, httptest.NewRequest("GET", a.URL, nil))
	if w.Code != 200 || !bytes.Equal(w.Body.Bytes(), raw) || w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("round trip", w.Code)
	}
	for _, p := range []string{"../secret", "/", "wrong.png"} {
		w = httptest.NewRecorder()
		s.ServeHTTP(w, httptest.NewRequest("GET", "/dikasa/v1/avatar-images/"+p, nil))
		if w.Code != 404 {
			t.Fatal("unsafe path", p)
		}
	}
	if e = s.Delete(context.Background(), "../secret"); e == nil {
		t.Fatal("unsafe delete accepted")
	}
	if e = s.Delete(context.Background(), a.ObjectName); e != nil {
		t.Fatal(e)
	}
	w = httptest.NewRecorder()
	s.ServeHTTP(w, httptest.NewRequest("GET", a.URL, nil))
	if w.Code != 404 {
		t.Fatal("deleted file still available")
	}
}
func TestAvatarRejectsHugeDimensionsAndFalseMIME(t *testing.T) {
	s, _ := NewLocalAvatarStore(t.TempDir(), "http://localhost")
	defer s.Close()
	var b bytes.Buffer
	png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 4097, 1)))
	raw := b.Bytes()
	_, e := s.Save(context.Background(), "11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222", &AvatarUpload{Reader: bytes.NewReader(raw), Size: int64(len(raw)), ContentType: "image/png", Filename: "a.png"})
	if !errors.Is(e, ErrInvalidAvatar) {
		t.Fatal("huge dimensions accepted", e)
	}
	_, e = s.Save(context.Background(), "x", "y", &AvatarUpload{Reader: strings.NewReader("fake"), Size: 4, ContentType: "image/png", Filename: "a.png"})
	if !errors.Is(e, ErrInvalidAvatar) {
		t.Fatal("fake accepted", e)
	}
}
