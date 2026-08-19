package catalog

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"be-dikasa/internal/user"
)

const maxSearchLength = 100

// AccountFinder mengambil outlet dan role user untuk otorisasi katalog.
type AccountFinder interface {
	FindByIDWithRole(context.Context, string) (*user.User, error)
}

// Service menyediakan aturan bisnis katalog yang bebas transport.
type Service struct {
	repository Repository
	accounts   AccountFinder
	images     ImageStore
}

// NewService membuat service katalog.
func NewService(repository Repository, accounts AccountFinder, images ImageStore) *Service {
	if images == nil {
		images = NewDisabledImageStore()
	}
	return &Service{repository: repository, accounts: accounts, images: images}
}

func (service *Service) scope(ctx context.Context, userID string, managerOnly bool) (string, error) {
	account, err := service.accounts.FindByIDWithRole(ctx, userID)
	if err != nil {
		if errors.Is(err, user.ErrNotFound) {
			return "", ErrNotFound
		}
		return "", err
	}
	if account.OutletID == nil || strings.TrimSpace(*account.OutletID) == "" {
		return "", ErrUserWithoutOutlet
	}
	if managerOnly && !strings.EqualFold(account.Role.Name, "manager") {
		return "", ErrForbidden
	}
	return *account.OutletID, nil
}

// ListProductTypes mengambil tipe produk global yang aktif.
func (service *Service) ListProductTypes(ctx context.Context, userID string) ([]ProductType, error) {
	if _, err := service.scope(ctx, userID, false); err != nil {
		return nil, err
	}
	return service.repository.ListProductTypes(ctx)
}

// ListProducts mengambil produk outlet dengan pencarian nama dan pagination.
func (service *Service) ListProducts(ctx context.Context, userID string, filter ProductFilter) ([]Product, int64, error) {
	outletID, err := service.scope(ctx, userID, filter.IncludeInactive)
	if err != nil {
		return nil, 0, err
	}
	filter.SearchByProductName = strings.TrimSpace(filter.SearchByProductName)
	if len([]rune(filter.SearchByProductName)) > maxSearchLength {
		return nil, 0, ErrValidation
	}
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 {
		filter.PageSize = 30
	}
	if filter.PageSize > 100 {
		return nil, 0, ErrValidation
	}
	filter.OutletID = outletID
	return service.repository.ListProducts(ctx, filter)
}

// GetProduct mengambil detail produk yang terisolasi pada outlet user.
func (service *Service) GetProduct(ctx context.Context, userID, productID string, includeInactive bool) (*Product, error) {
	outletID, err := service.scope(ctx, userID, includeInactive)
	if err != nil {
		return nil, err
	}
	return service.repository.FindProduct(ctx, outletID, productID, includeInactive)
}

// CreateProduct membuat produk baru setelah kategori dan gambar tervalidasi.
func (service *Service) CreateProduct(ctx context.Context, userID string, mutation ProductMutation) (*Product, error) {
	outletID, err := service.scope(ctx, userID, true)
	if err != nil {
		return nil, err
	}
	mutation.Name = strings.TrimSpace(mutation.Name)
	if mutation.Name == "" || mutation.CategoryID == "" || mutation.Price < 0 {
		return nil, ErrValidation
	}
	category, err := service.repository.FindCategory(ctx, outletID, mutation.CategoryID)
	if err != nil {
		return nil, err
	}
	if !category.IsActive || (mutation.ProductType != "" && !strings.EqualFold(category.ProductType.Code, mutation.ProductType)) {
		return nil, ErrCategoryMismatch
	}
	product := &Product{ID: uuid.NewString(), OutletID: outletID, CategoryID: category.ID, Name: mutation.Name, Price: mutation.Price,
		Description: strings.TrimSpace(mutation.Description), IsHighlighted: mutation.IsHighlighted, SortOrder: mutation.SortOrder, IsActive: true, Category: *category}
	if mutation.Image != nil {
		stored, saveErr := service.images.Save(ctx, outletID, product.ID, mutation.Image)
		if saveErr != nil {
			return nil, saveErr
		}
		product.ImageURL, product.ImageObjectName = &stored.URL, &stored.ObjectName
		if err = service.repository.CreateProduct(ctx, product); err != nil {
			_ = service.images.Delete(ctx, stored.ObjectName)
			return nil, err
		}
		return product, nil
	}
	if err = service.repository.CreateProduct(ctx, product); err != nil {
		return nil, err
	}
	return product, nil
}

// UpdateProduct mengganti data produk dan memastikan object lama baru dihapus setelah commit database berhasil.
func (service *Service) UpdateProduct(ctx context.Context, userID, productID string, mutation ProductMutation) (*Product, error) {
	outletID, err := service.scope(ctx, userID, true)
	if err != nil {
		return nil, err
	}
	product, err := service.repository.FindProduct(ctx, outletID, productID, true)
	if err != nil {
		return nil, err
	}
	isSet := func(field string) bool { return mutation.Fields == nil || mutation.Fields[field] }
	categoryID := product.CategoryID
	if isSet("category_id") {
		categoryID = mutation.CategoryID
	}
	category, err := service.repository.FindCategory(ctx, outletID, categoryID)
	if err != nil {
		return nil, err
	}
	if !category.IsActive || (mutation.ProductType != "" && !strings.EqualFold(category.ProductType.Code, mutation.ProductType)) {
		return nil, ErrCategoryMismatch
	}
	oldObject := product.ImageObjectName
	var uploaded *StoredImage
	if mutation.Image != nil {
		uploaded, err = service.images.Save(ctx, outletID, product.ID, mutation.Image)
		if err != nil {
			return nil, err
		}
		product.ImageURL, product.ImageObjectName = &uploaded.URL, &uploaded.ObjectName
	} else if isSet("remove_image") && mutation.RemoveImage {
		product.ImageURL, product.ImageObjectName = nil, nil
	}
	if isSet("name") {
		product.Name = strings.TrimSpace(mutation.Name)
	}
	product.CategoryID, product.Category = category.ID, *category
	if isSet("price") {
		product.Price = mutation.Price
	}
	if isSet("description") {
		product.Description = strings.TrimSpace(mutation.Description)
	}
	if isSet("is_highlighted") {
		product.IsHighlighted = mutation.IsHighlighted
	}
	if isSet("sort_order") {
		product.SortOrder = mutation.SortOrder
	}
	if isSet("is_active") {
		product.IsActive = mutation.IsActive
	}
	if product.Name == "" || product.Price < 0 {
		if uploaded != nil {
			_ = service.images.Delete(ctx, uploaded.ObjectName)
		}
		return nil, ErrValidation
	}
	if err = service.repository.UpdateProduct(ctx, product); err != nil {
		if uploaded != nil {
			_ = service.images.Delete(ctx, uploaded.ObjectName)
		}
		return nil, err
	}
	if oldObject != nil && (uploaded != nil || (isSet("remove_image") && mutation.RemoveImage)) {
		_ = service.images.Delete(ctx, *oldObject)
	}
	return product, nil
}

// DeleteProduct melakukan soft delete tanpa menghapus gambar.
func (service *Service) DeleteProduct(ctx context.Context, userID, productID string) error {
	outletID, err := service.scope(ctx, userID, true)
	if err != nil {
		return err
	}
	return service.repository.SoftDeleteProduct(ctx, outletID, productID)
}

// ListCategories mengambil dropdown kategori sesuai tipe dan outlet.
func (service *Service) ListCategories(ctx context.Context, userID, productType string, includeInactive bool) ([]ProductCategory, error) {
	outletID, err := service.scope(ctx, userID, includeInactive)
	if err != nil {
		return nil, err
	}
	return service.repository.ListCategories(ctx, outletID, productType, includeInactive)
}

// CreateCategory membuat kategori outlet untuk tipe produk global.
func (service *Service) CreateCategory(ctx context.Context, userID, productType string, mutation CategoryMutation) (*ProductCategory, error) {
	outletID, err := service.scope(ctx, userID, true)
	if err != nil {
		return nil, err
	}
	typeModel, err := service.repository.FindProductType(ctx, productType)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(mutation.Name) == "" {
		return nil, ErrValidation
	}
	category := &ProductCategory{ID: uuid.NewString(), OutletID: outletID, ProductTypeID: typeModel.ID, ProductType: *typeModel, Name: strings.TrimSpace(mutation.Name), SortOrder: mutation.SortOrder, IsActive: true}
	if err = service.repository.CreateCategory(ctx, category); err != nil {
		return nil, err
	}
	return category, nil
}

// UpdateCategory memperbarui kategori dan mencegah nonaktif bila masih dipakai produk aktif.
func (service *Service) UpdateCategory(ctx context.Context, userID, productType, categoryID string, mutation CategoryMutation) (*ProductCategory, error) {
	outletID, err := service.scope(ctx, userID, true)
	if err != nil {
		return nil, err
	}
	category, err := service.repository.FindCategory(ctx, outletID, categoryID)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(category.ProductType.Code, productType) {
		return nil, ErrNotFound
	}
	if !mutation.IsActive {
		used, checkErr := service.repository.CategoryHasActiveProducts(ctx, outletID, categoryID)
		if checkErr != nil {
			return nil, checkErr
		}
		if used {
			return nil, ErrCategoryInUse
		}
	}
	if strings.TrimSpace(mutation.Name) == "" {
		return nil, ErrValidation
	}
	category.Name, category.SortOrder, category.IsActive = strings.TrimSpace(mutation.Name), mutation.SortOrder, mutation.IsActive
	if err = service.repository.UpdateCategory(ctx, category); err != nil {
		return nil, err
	}
	return category, nil
}

// DeleteCategory menonaktifkan kategori yang tidak lagi dipakai produk aktif.
func (service *Service) DeleteCategory(ctx context.Context, userID, productType, categoryID string) error {
	outletID, err := service.scope(ctx, userID, true)
	if err != nil {
		return err
	}
	category, err := service.repository.FindCategory(ctx, outletID, categoryID)
	if err != nil {
		return err
	}
	if !strings.EqualFold(category.ProductType.Code, productType) {
		return ErrNotFound
	}
	used, err := service.repository.CategoryHasActiveProducts(ctx, outletID, categoryID)
	if err != nil {
		return err
	}
	if used {
		return ErrCategoryInUse
	}
	return service.repository.SoftDeleteCategory(ctx, outletID, categoryID)
}

// ListOrderTypes mengambil pilihan jenis order outlet.
func (service *Service) ListOrderTypes(ctx context.Context, userID string, includeInactive bool) ([]OrderType, error) {
	outletID, err := service.scope(ctx, userID, includeInactive)
	if err != nil {
		return nil, err
	}
	return service.repository.ListOrderTypes(ctx, outletID, includeInactive)
}

// CreateOrderType membuat jenis order outlet.
func (service *Service) CreateOrderType(ctx context.Context, userID string, mutation OrderTypeMutation) (*OrderType, error) {
	outletID, err := service.scope(ctx, userID, true)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(mutation.Code) == "" || strings.TrimSpace(mutation.Name) == "" {
		return nil, ErrValidation
	}
	value := &OrderType{ID: uuid.NewString(), OutletID: outletID, Code: strings.TrimSpace(mutation.Code), Name: strings.TrimSpace(mutation.Name), IconKey: mutation.IconKey, SortOrder: mutation.SortOrder, IsActive: true}
	if err = service.repository.CreateOrderType(ctx, value); err != nil {
		return nil, err
	}
	return value, nil
}

// UpdateOrderType memperbarui jenis order outlet.
func (service *Service) UpdateOrderType(ctx context.Context, userID, id string, mutation OrderTypeMutation) (*OrderType, error) {
	outletID, err := service.scope(ctx, userID, true)
	if err != nil {
		return nil, err
	}
	value, err := service.repository.FindOrderType(ctx, outletID, id)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(mutation.Code) == "" || strings.TrimSpace(mutation.Name) == "" {
		return nil, ErrValidation
	}
	value.Code, value.Name, value.IconKey, value.SortOrder, value.IsActive = strings.TrimSpace(mutation.Code), strings.TrimSpace(mutation.Name), mutation.IconKey, mutation.SortOrder, mutation.IsActive
	if err = service.repository.UpdateOrderType(ctx, value); err != nil {
		return nil, err
	}
	return value, nil
}

// DeleteOrderType melakukan soft delete jenis order outlet.
func (service *Service) DeleteOrderType(ctx context.Context, userID, id string) error {
	outletID, err := service.scope(ctx, userID, true)
	if err != nil {
		return err
	}
	return service.repository.SoftDeleteOrderType(ctx, outletID, id)
}
