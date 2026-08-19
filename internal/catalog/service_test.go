package catalog

import (
	"context"
	"errors"
	"testing"

	"be-dikasa/internal/user"
)

type fakeRepository struct {
	listFilter ProductFilter
	products   []Product
	product    *Product
	category   *ProductCategory
	created    *Product
	updated    *Product
	hasActive  bool
	err        error
	createErr  error
}

func (repository *fakeRepository) ListProductTypes(context.Context) ([]ProductType, error) {
	return nil, repository.err
}
func (repository *fakeRepository) ListProducts(_ context.Context, filter ProductFilter) ([]Product, int64, error) {
	repository.listFilter = filter
	return repository.products, int64(len(repository.products)), repository.err
}
func (repository *fakeRepository) FindProduct(context.Context, string, string, bool) (*Product, error) {
	return repository.product, repository.err
}
func (repository *fakeRepository) CreateProduct(_ context.Context, product *Product) error {
	repository.created = product
	return repository.createErr
}
func (repository *fakeRepository) UpdateProduct(_ context.Context, product *Product) error {
	repository.updated = product
	return repository.err
}
func (repository *fakeRepository) SoftDeleteProduct(context.Context, string, string) error {
	return repository.err
}
func (repository *fakeRepository) FindCategory(context.Context, string, string) (*ProductCategory, error) {
	return repository.category, repository.err
}
func (repository *fakeRepository) ListCategories(context.Context, string, string, bool) ([]ProductCategory, error) {
	return nil, repository.err
}
func (repository *fakeRepository) CreateCategory(context.Context, *ProductCategory) error {
	return repository.err
}
func (repository *fakeRepository) UpdateCategory(context.Context, *ProductCategory) error {
	return repository.err
}
func (repository *fakeRepository) SoftDeleteCategory(context.Context, string, string) error {
	return repository.err
}
func (repository *fakeRepository) CategoryHasActiveProducts(context.Context, string, string) (bool, error) {
	return repository.hasActive, repository.err
}
func (repository *fakeRepository) FindProductType(context.Context, string) (*ProductType, error) {
	return &ProductType{ID: "type-id", Code: "food", Name: "Makanan", IsActive: true}, repository.err
}
func (repository *fakeRepository) ListOrderTypes(context.Context, string, bool) ([]OrderType, error) {
	return nil, repository.err
}
func (repository *fakeRepository) FindOrderType(context.Context, string, string) (*OrderType, error) {
	return nil, repository.err
}
func (repository *fakeRepository) CreateOrderType(context.Context, *OrderType) error {
	return repository.err
}
func (repository *fakeRepository) UpdateOrderType(context.Context, *OrderType) error {
	return repository.err
}
func (repository *fakeRepository) SoftDeleteOrderType(context.Context, string, string) error {
	return repository.err
}

type fakeAccountFinder struct {
	account *user.User
	err     error
}

type fakeImageStore struct {
	stored  StoredImage
	saved   int
	deleted []string
	err     error
}

func (store *fakeImageStore) Save(context.Context, string, string, *ImageUpload) (*StoredImage, error) {
	store.saved++
	if store.err != nil {
		return nil, store.err
	}
	return &store.stored, nil
}
func (store *fakeImageStore) Delete(_ context.Context, name string) error {
	store.deleted = append(store.deleted, name)
	return store.err
}

func (finder fakeAccountFinder) FindByIDWithRole(context.Context, string) (*user.User, error) {
	return finder.account, finder.err
}

func account(role string) *user.User {
	outletID := "outlet-a"
	return &user.User{ID: "user-a", OutletID: &outletID, Role: user.Role{Name: role}}
}

func TestListProductsNormalizesSearchAndScopesOutlet(t *testing.T) {
	repository := &fakeRepository{}
	service := NewService(repository, fakeAccountFinder{account: account("cashier")}, NewDisabledImageStore())

	_, _, err := service.ListProducts(context.Background(), "user-a", ProductFilter{
		ProductType: "food", SearchByProductName: "  CuMi%_  ", Page: 2, PageSize: 30,
	})
	if err != nil {
		t.Fatalf("list products: %v", err)
	}
	if repository.listFilter.OutletID != "outlet-a" || repository.listFilter.SearchByProductName != "CuMi%_" {
		t.Fatalf("unexpected normalized filter: %+v", repository.listFilter)
	}
}

func TestListProductsRejectsLongSearchAndCashierInactiveAccess(t *testing.T) {
	service := NewService(&fakeRepository{}, fakeAccountFinder{account: account("cashier")}, NewDisabledImageStore())
	if _, _, err := service.ListProducts(context.Background(), "user-a", ProductFilter{SearchByProductName: string(make([]byte, 101)), Page: 1, PageSize: 30}); !errors.Is(err, ErrValidation) {
		t.Fatalf("expected search validation, got %v", err)
	}
	if _, _, err := service.ListProducts(context.Background(), "user-a", ProductFilter{IncludeInactive: true, Page: 1, PageSize: 30}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected manager-only inactive list, got %v", err)
	}
}

func TestCreateProductRequiresManagerAndMatchingCategory(t *testing.T) {
	repository := &fakeRepository{category: &ProductCategory{ID: "cat", OutletID: "outlet-a", ProductType: ProductType{Code: "drink"}, IsActive: true}}
	cashierService := NewService(repository, fakeAccountFinder{account: account("cashier")}, NewDisabledImageStore())
	if _, err := cashierService.CreateProduct(context.Background(), "user-a", ProductMutation{Name: "Teh", CategoryID: "cat", Price: 5000}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
	managerService := NewService(repository, fakeAccountFinder{account: account("manager")}, NewDisabledImageStore())
	if _, err := managerService.CreateProduct(context.Background(), "user-a", ProductMutation{Name: "Teh", CategoryID: "cat", ProductType: "food", Price: 5000}); !errors.Is(err, ErrCategoryMismatch) {
		t.Fatalf("expected category mismatch, got %v", err)
	}
}

func TestCreateProductStorageDisabledOnlyRejectsImage(t *testing.T) {
	repository := &fakeRepository{category: &ProductCategory{ID: "cat", OutletID: "outlet-a", ProductType: ProductType{Code: "food"}, IsActive: true}}
	service := NewService(repository, fakeAccountFinder{account: account("manager")}, NewDisabledImageStore())
	if _, err := service.CreateProduct(context.Background(), "user-a", ProductMutation{Name: "Nasi", CategoryID: "cat", ProductType: "food", Price: 12000}); err != nil {
		t.Fatalf("image-less product should work: %v", err)
	}
	if _, err := service.CreateProduct(context.Background(), "user-a", ProductMutation{Name: "Nasi", CategoryID: "cat", ProductType: "food", Price: 12000, Image: &ImageUpload{}}); !errors.Is(err, ErrStorageDisabled) {
		t.Fatalf("expected disabled storage, got %v", err)
	}
}

func TestCreateProductStoresCommerceFieldsAndVariations(t *testing.T) {
	repository := &fakeRepository{category: &ProductCategory{ID: "cat", OutletID: "outlet-a", ProductType: ProductType{Code: "food"}, IsActive: true}}
	service := NewService(repository, fakeAccountFinder{account: account("manager")}, NewDisabledImageStore())
	onlinePrice, costPrice := int64(25_000), int64(12_000)
	created, err := service.CreateProduct(context.Background(), "user-a", ProductMutation{
		Name: "Nasi Goreng", CategoryID: "cat", ProductType: "food", Price: 21_000,
		OnlineSellingPrice: &onlinePrice, CostPrice: &costPrice, StockStatus: StockStatusAvailable,
		Variations: []ProductVariantMutation{{Name: "Jumbo", Price: 28_000, StockStatus: StockStatusAvailable}},
	})
	if err != nil {
		t.Fatalf("create commerce product: %v", err)
	}
	if created.OnlineSellingPrice == nil || *created.OnlineSellingPrice != onlinePrice || created.StockStatus != StockStatusAvailable || len(created.Variants) != 1 || created.Variants[0].OutletID != "outlet-a" {
		t.Fatalf("unexpected product fields: %+v", created)
	}
}

func TestCreateProductCleansNewImageWhenDatabaseFails(t *testing.T) {
	repository := &fakeRepository{category: &ProductCategory{ID: "cat", OutletID: "outlet-a", ProductType: ProductType{Code: "food"}, IsActive: true}, createErr: errors.New("database unavailable")}
	images := &fakeImageStore{stored: StoredImage{URL: "https://image", ObjectName: "new-object"}}
	service := NewService(repository, fakeAccountFinder{account: account("manager")}, images)
	_, err := service.CreateProduct(context.Background(), "user-a", ProductMutation{Name: "Nasi", CategoryID: "cat", Price: 12000, Image: &ImageUpload{}})
	if err == nil || len(images.deleted) != 1 || images.deleted[0] != "new-object" {
		t.Fatalf("expected failed DB upload cleanup, err=%v deleted=%v", err, images.deleted)
	}
}

func TestDeleteCategoryRejectsCategoryWithActiveProducts(t *testing.T) {
	repository := &fakeRepository{category: &ProductCategory{ID: "cat", OutletID: "outlet-a", ProductType: ProductType{Code: "food"}, IsActive: true}, hasActive: true}
	service := NewService(repository, fakeAccountFinder{account: account("manager")}, NewDisabledImageStore())
	if err := service.DeleteCategory(context.Background(), "user-a", "food", "cat"); !errors.Is(err, ErrCategoryInUse) {
		t.Fatalf("expected category conflict, got %v", err)
	}
}

func TestUpdateProductReplacesAndRemovesImagesAfterDatabaseSuccess(t *testing.T) {
	oldURL, oldObject := "https://old", "old-object"
	baseProduct := &Product{ID: "product-a", OutletID: "outlet-a", CategoryID: "cat", Name: "Nasi", Price: 12000, ImageURL: &oldURL, ImageObjectName: &oldObject, IsActive: true}
	category := &ProductCategory{ID: "cat", OutletID: "outlet-a", ProductType: ProductType{Code: "food"}, IsActive: true}
	repository := &fakeRepository{product: baseProduct, category: category}
	images := &fakeImageStore{stored: StoredImage{URL: "https://new", ObjectName: "new-object"}}
	service := NewService(repository, fakeAccountFinder{account: account("manager")}, images)

	updated, err := service.UpdateProduct(context.Background(), "user-a", "product-a", ProductMutation{Name: "Nasi Baru", CategoryID: "cat", Price: 13000, IsActive: true, Image: &ImageUpload{}})
	if err != nil || updated.ImageObjectName == nil || *updated.ImageObjectName != "new-object" || len(images.deleted) != 1 || images.deleted[0] != "old-object" {
		t.Fatalf("replace image failed: product=%+v deleted=%v err=%v", updated, images.deleted, err)
	}

	newObject := "new-object"
	repository.product = &Product{ID: "product-a", OutletID: "outlet-a", CategoryID: "cat", Name: "Nasi Baru", Price: 13000, ImageObjectName: &newObject, IsActive: true}
	updated, err = service.UpdateProduct(context.Background(), "user-a", "product-a", ProductMutation{Name: "Nasi Baru", CategoryID: "cat", Price: 13000, IsActive: true, RemoveImage: true})
	if err != nil || updated.ImageObjectName != nil || images.deleted[len(images.deleted)-1] != "new-object" {
		t.Fatalf("remove image failed: product=%+v deleted=%v err=%v", updated, images.deleted, err)
	}
}

func TestUpdateProductSupportsPartialMutation(t *testing.T) {
	category := &ProductCategory{ID: "cat", OutletID: "outlet-a", ProductType: ProductType{Code: "food"}, IsActive: true}
	repository := &fakeRepository{product: &Product{ID: "product-a", OutletID: "outlet-a", CategoryID: "cat", Name: "Nasi", Price: 12000, Description: "lama", IsActive: true, Category: *category}, category: category}
	service := NewService(repository, fakeAccountFinder{account: account("manager")}, NewDisabledImageStore())

	updated, err := service.UpdateProduct(context.Background(), "user-a", "product-a", ProductMutation{Description: "baru", Fields: map[string]bool{"description": true}})
	if err != nil {
		t.Fatalf("partial update: %v", err)
	}
	if updated.Name != "Nasi" || updated.Price != 12000 || updated.CategoryID != "cat" || updated.Description != "baru" || !updated.IsActive {
		t.Fatalf("partial update overwrote omitted fields: %+v", updated)
	}
}
