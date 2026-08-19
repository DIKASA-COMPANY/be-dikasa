package catalog

import "context"

type disabledImageStore struct{}

// NewDisabledImageStore membuat storage yang menolak upload tanpa mengganggu produk tanpa gambar.
func NewDisabledImageStore() ImageStore { return disabledImageStore{} }

func (disabledImageStore) Save(context.Context, string, string, *ImageUpload) (*StoredImage, error) {
	return nil, ErrStorageDisabled
}
func (disabledImageStore) Delete(context.Context, string) error { return ErrStorageDisabled }
