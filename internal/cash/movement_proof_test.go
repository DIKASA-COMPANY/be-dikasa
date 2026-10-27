package cash

import (
	"be-dikasa/internal/shared/imagefile"
	"bytes"
	"context"
	"image"
	"image/png"
	"testing"
)

func TestMovementProofPrivateConfinement(t *testing.T) {
	var b bytes.Buffer
	png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 2, 2)))
	raw := b.Bytes()
	v, e := imagefile.Validate(imagefile.Upload{Reader: bytes.NewReader(raw), Size: int64(len(raw)), ContentType: "image/png", Filename: "p.png"}, imagefile.AvatarPolicy)
	if e != nil {
		t.Fatal(e)
	}
	s, e := NewLocalMovementProofStore(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	o, e := s.Save(context.Background(), "11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222", v)
	if e != nil {
		t.Fatal(e)
	}
	data, mime, e := s.Read(context.Background(), o.Key)
	if e != nil || mime != "image/png" || !bytes.Equal(raw, data) {
		t.Fatal("proof changed", e)
	}
	if _, _, e = s.Read(context.Background(), "../secret"); e == nil {
		t.Fatal("unsafe read")
	}
	if e = s.Delete(context.Background(), "../secret"); e == nil {
		t.Fatal("unsafe delete")
	}
	if e = s.Delete(context.Background(), o.Key); e != nil {
		t.Fatal(e)
	}
	if _, _, e = s.Read(context.Background(), o.Key); e == nil {
		t.Fatal("missing proof accepted")
	}
}
