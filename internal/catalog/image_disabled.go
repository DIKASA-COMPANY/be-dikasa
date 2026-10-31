package catalog

import (
	"be-dikasa/internal/shared/supabasestorage"
	"context"
)

type disabledImageStore struct{}

// NewDisabledImageStore membuat storage yang menolak upload tanpa mengganggu produk tanpa gambar.
func NewDisabledImageStore() ImageStore { return disabledImageStore{} }

func (disabledImageStore) Prepare(context.Context, string, string, string, int64) (supabasestorage.SignedUpload, error) {
	return supabasestorage.SignedUpload{}, ErrStorageDisabled
}
func (disabledImageStore) Adopt(context.Context, string, string, string) (*StoredImage, error) {
	return nil, ErrStorageDisabled
}
func (disabledImageStore) Delete(context.Context, string) error { return ErrStorageDisabled }
