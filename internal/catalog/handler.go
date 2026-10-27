package catalog

import (
	"context"
	"errors"
	"net/http"

	sharederrors "be-dikasa/internal/shared/errors"
	sharedvalidator "be-dikasa/internal/shared/validator"
	"be-dikasa/internal/user"
)

// Provider mendefinisikan use case katalog untuk HTTP handler.
type Provider interface {
	ListProductTypes(context.Context, string) ([]ProductType, error)
	ListProducts(context.Context, string, ProductFilter) ([]Product, int64, error)
	GetProduct(context.Context, string, string, bool) (*Product, error)
	CreateProduct(context.Context, string, ProductMutation) (*Product, error)
	UpdateProduct(context.Context, string, string, ProductMutation) (*Product, error)
	DeleteProduct(context.Context, string, string) error
	ListCategories(context.Context, string, string, bool) ([]ProductCategory, error)
	CreateCategory(context.Context, string, string, CategoryMutation) (*ProductCategory, error)
	UpdateCategory(context.Context, string, string, string, CategoryMutation) (*ProductCategory, error)
	DeleteCategory(context.Context, string, string, string) error
	ListOrderTypes(context.Context, string, bool) ([]OrderType, error)
	CreateOrderType(context.Context, string, OrderTypeMutation) (*OrderType, error)
	UpdateOrderType(context.Context, string, string, OrderTypeMutation) (*OrderType, error)
	DeleteOrderType(context.Context, string, string) error
}

// Handler melayani endpoint katalog.
type Handler struct {
	provider  Provider
	validator *sharedvalidator.Validator
}

// NewHandler membuat handler katalog.
func NewHandler(provider Provider, validator *sharedvalidator.Validator) *Handler {
	return &Handler{provider: provider, validator: validator}
}

func authenticated(ctx context.Context) (string, error) {
	value, ok := user.AuthenticatedUserIDFromContext(ctx)
	if !ok {
		return "", sharederrors.Standard(http.StatusUnauthorized, nil)
	}
	return value, nil
}

// ListProductTypes melayani daftar master tipe produk aktif.
func (handler *Handler) ListProductTypes(ctx context.Context, _ *struct{}) (*ProductTypesOutput, error) {
	userID, err := authenticated(ctx)
	if err != nil {
		return nil, err
	}
	values, err := handler.provider.ListProductTypes(ctx, userID)
	if err != nil {
		return nil, catalogStatusError(err)
	}
	output := &ProductTypesOutput{}
	output.Body.Data = make([]ProductTypeData, 0, len(values))
	for _, value := range values {
		output.Body.Data = append(output.Body.Data, productTypeData(value))
	}
	return output, nil
}

// ListProducts melayani pencarian dan pagination produk outlet.
func (handler *Handler) ListProducts(ctx context.Context, input *ListProductsInput) (*ProductsOutput, error) {
	userID, err := authenticated(ctx)
	if err != nil {
		return nil, err
	}
	filter := ProductFilter{ProductType: input.ProductType, CategoryID: input.CategoryID, SearchByProductName: input.SearchByProductName, Page: input.Page, PageSize: input.PageSize, IncludeInactive: input.IncludeInactive}
	values, total, err := handler.provider.ListProducts(ctx, userID, filter)
	if err != nil {
		return nil, catalogStatusError(err)
	}
	output := &ProductsOutput{}
	output.Body.Data = productsData(values)
	output.Body.Meta.Page, output.Body.Meta.PageSize, output.Body.Meta.Total = filter.Page, filter.PageSize, total
	return output, nil
}

// GetProduct melayani detail produk pada scope outlet.
func (handler *Handler) GetProduct(ctx context.Context, input *ProductIDInput) (*ProductOutput, error) {
	userID, err := authenticated(ctx)
	if err != nil {
		return nil, err
	}
	value, err := handler.provider.GetProduct(ctx, userID, input.ProductID, input.IncludeInactive)
	if err != nil {
		return nil, catalogStatusError(err)
	}
	output := &ProductOutput{}
	output.Body.Data = productData(*value)
	return output, nil
}

func productMutation(input *ProductCreateInput) ProductMutation {
	form := input.RawBody.Data()
	mutation := ProductMutation{Name: form.Name, CategoryID: form.CategoryID, ProductType: form.ProductType, Price: form.SellingPrice,
		StockStatus: form.StockStatus, Variations: variantMutations(form.Variations), Description: form.Description,
		IsHighlighted: form.IsHighlighted, SortOrder: form.SortOrder, IsActive: form.IsActive, RemoveImage: form.RemoveImage}
	if input.RawBody.Form != nil {
		if _, ok := input.RawBody.Form.Value["online_selling_price"]; ok {
			mutation.OnlineSellingPrice = &form.OnlineSellingPrice
		}
		if _, ok := input.RawBody.Form.Value["cost_price"]; ok {
			mutation.CostPrice = &form.CostPrice
		}
		if _, ok := input.RawBody.Form.Value["online_cost_price"]; ok {
			mutation.OnlineCostPrice = &form.OnlineCostPrice
		}
	}
	if form.Image.IsSet {
		mutation.Image = &ImageUpload{Reader: form.Image.File, Size: form.Image.Size, ContentType: form.Image.ContentType, Filename: form.Image.Filename}
	}
	return mutation
}

func productPatchMutation(input *ProductMutationInput) ProductMutation {
	form := input.RawBody.Data()
	fields := make(map[string]bool)
	if input.RawBody.Form != nil {
		for key := range input.RawBody.Form.Value {
			fields[key] = true
		}
	}
	mutation := ProductMutation{Name: form.Name, CategoryID: form.CategoryID, ProductType: form.ProductType, Price: form.SellingPrice,
		StockStatus: form.StockStatus, Variations: variantMutations(form.Variations), Description: form.Description,
		IsHighlighted: form.IsHighlighted, SortOrder: form.SortOrder, IsActive: form.IsActive, RemoveImage: form.RemoveImage, Fields: fields}
	if fields["online_selling_price"] {
		mutation.OnlineSellingPrice = &form.OnlineSellingPrice
	}
	if fields["cost_price"] {
		mutation.CostPrice = &form.CostPrice
	}
	if fields["online_cost_price"] {
		mutation.OnlineCostPrice = &form.OnlineCostPrice
	}
	if fields["selling_price"] {
		fields["price"] = true
	}
	if form.Image.IsSet {
		mutation.Image = &ImageUpload{Reader: form.Image.File, Size: form.Image.Size, ContentType: form.Image.ContentType, Filename: form.Image.Filename}
	}
	return mutation
}

// CreateProduct melayani pembuatan produk multipart oleh manager.
func (handler *Handler) CreateProduct(ctx context.Context, input *ProductCreateInput) (*ProductOutput, error) {
	userID, err := authenticated(ctx)
	if err != nil {
		return nil, err
	}
	value, err := handler.provider.CreateProduct(ctx, userID, productMutation(input))
	if err != nil {
		return nil, catalogStatusError(err)
	}
	output := &ProductOutput{Status: http.StatusCreated}
	output.Body.Data = productData(*value)
	return output, nil
}

// UpdateProduct melayani perubahan produk dan gambar oleh manager.
func (handler *Handler) UpdateProduct(ctx context.Context, input *ProductMutationInput) (*ProductOutput, error) {
	userID, err := authenticated(ctx)
	if err != nil {
		return nil, err
	}
	value, err := handler.provider.UpdateProduct(ctx, userID, input.ProductID, productPatchMutation(input))
	if err != nil {
		return nil, catalogStatusError(err)
	}
	output := &ProductOutput{}
	output.Body.Data = productData(*value)
	return output, nil
}

// DeleteProduct melayani soft delete produk oleh manager.
func (handler *Handler) DeleteProduct(ctx context.Context, input *ProductDeleteInput) (*NoContentOutput, error) {
	userID, err := authenticated(ctx)
	if err != nil {
		return nil, err
	}
	if err = handler.provider.DeleteProduct(ctx, userID, input.ProductID); err != nil {
		return nil, catalogStatusError(err)
	}
	return &NoContentOutput{Status: http.StatusNoContent}, nil
}

// ListCategories melayani dropdown kategori sesuai tipe produk.
func (handler *Handler) ListCategories(ctx context.Context, input *ProductTypePathInput) (*CategoriesOutput, error) {
	userID, err := authenticated(ctx)
	if err != nil {
		return nil, err
	}
	values, err := handler.provider.ListCategories(ctx, userID, input.ProductType, input.IncludeInactive)
	if err != nil {
		return nil, catalogStatusError(err)
	}
	output := &CategoriesOutput{}
	output.Body.Data = make([]CategoryData, 0, len(values))
	for _, value := range values {
		output.Body.Data = append(output.Body.Data, categoryData(value))
	}
	return output, nil
}

// CreateCategory melayani pembuatan kategori oleh manager.
func (handler *Handler) CreateCategory(ctx context.Context, input *CategoryCreateInput) (*CategoryOutput, error) {
	userID, err := authenticated(ctx)
	if err != nil {
		return nil, err
	}
	if details := handler.validator.Validate(input.Body); len(details) > 0 {
		return nil, sharederrors.New(422, "VALIDASI_GAGAL", "Data yang dikirim tidak memenuhi ketentuan.", details)
	}
	value, err := handler.provider.CreateCategory(ctx, userID, input.ProductType, CategoryMutation{Name: input.Body.Name, SortOrder: input.Body.SortOrder, IsActive: true})
	if err != nil {
		return nil, catalogStatusError(err)
	}
	output := &CategoryOutput{Status: 201}
	output.Body.Data = categoryData(*value)
	return output, nil
}

// UpdateCategory melayani perubahan kategori oleh manager.
func (handler *Handler) UpdateCategory(ctx context.Context, input *CategoryMutationInput) (*CategoryOutput, error) {
	userID, err := authenticated(ctx)
	if err != nil {
		return nil, err
	}
	if details := handler.validator.Validate(input.Body); len(details) > 0 {
		return nil, sharederrors.New(422, "VALIDASI_GAGAL", "Data yang dikirim tidak memenuhi ketentuan.", details)
	}
	value, err := handler.provider.UpdateCategory(ctx, userID, input.ProductType, input.CategoryID, CategoryMutation{Name: input.Body.Name, SortOrder: input.Body.SortOrder, IsActive: input.Body.IsActive})
	if err != nil {
		return nil, catalogStatusError(err)
	}
	output := &CategoryOutput{}
	output.Body.Data = categoryData(*value)
	return output, nil
}

// DeleteCategory melayani soft delete kategori oleh manager.
func (handler *Handler) DeleteCategory(ctx context.Context, input *CategoryDeleteInput) (*NoContentOutput, error) {
	userID, err := authenticated(ctx)
	if err != nil {
		return nil, err
	}
	if err = handler.provider.DeleteCategory(ctx, userID, input.ProductType, input.CategoryID); err != nil {
		return nil, catalogStatusError(err)
	}
	return &NoContentOutput{Status: 204}, nil
}

// ListOrderTypes melayani dropdown jenis order outlet.
func (handler *Handler) ListOrderTypes(ctx context.Context, input *ListOrderTypesInput) (*OrderTypesOutput, error) {
	userID, err := authenticated(ctx)
	if err != nil {
		return nil, err
	}
	values, err := handler.provider.ListOrderTypes(ctx, userID, input.IncludeInactive)
	if err != nil {
		return nil, catalogStatusError(err)
	}
	output := &OrderTypesOutput{}
	output.Body.Data = make([]OrderTypeData, 0, len(values))
	for _, value := range values {
		output.Body.Data = append(output.Body.Data, orderTypeData(value))
	}
	return output, nil
}

// CreateOrderType melayani pembuatan jenis order oleh manager.
func (handler *Handler) CreateOrderType(ctx context.Context, input *OrderTypeCreateInput) (*OrderTypeOutput, error) {
	userID, err := authenticated(ctx)
	if err != nil {
		return nil, err
	}
	if details := handler.validator.Validate(input.Body); len(details) > 0 {
		return nil, sharederrors.New(422, "VALIDASI_GAGAL", "Data yang dikirim tidak memenuhi ketentuan.", details)
	}
	value, err := handler.provider.CreateOrderType(ctx, userID, orderMutation(input.Body))
	if err != nil {
		return nil, catalogStatusError(err)
	}
	output := &OrderTypeOutput{Status: 201}
	output.Body.Data = orderTypeData(*value)
	return output, nil
}

// UpdateOrderType melayani perubahan jenis order oleh manager.
func (handler *Handler) UpdateOrderType(ctx context.Context, input *OrderTypeMutationInput) (*OrderTypeOutput, error) {
	userID, err := authenticated(ctx)
	if err != nil {
		return nil, err
	}
	if details := handler.validator.Validate(input.Body); len(details) > 0 {
		return nil, sharederrors.New(422, "VALIDASI_GAGAL", "Data yang dikirim tidak memenuhi ketentuan.", details)
	}
	value, err := handler.provider.UpdateOrderType(ctx, userID, input.OrderTypeID, orderMutation(input.Body))
	if err != nil {
		return nil, catalogStatusError(err)
	}
	output := &OrderTypeOutput{}
	output.Body.Data = orderTypeData(*value)
	return output, nil
}

// DeleteOrderType melayani soft delete jenis order oleh manager.
func (handler *Handler) DeleteOrderType(ctx context.Context, input *OrderTypeDeleteInput) (*NoContentOutput, error) {
	userID, err := authenticated(ctx)
	if err != nil {
		return nil, err
	}
	if err = handler.provider.DeleteOrderType(ctx, userID, input.OrderTypeID); err != nil {
		return nil, catalogStatusError(err)
	}
	return &NoContentOutput{Status: 204}, nil
}

func orderMutation(value OrderTypeRequest) OrderTypeMutation {
	return OrderTypeMutation{Code: value.Code, Name: value.Name, IconKey: value.IconKey, SortOrder: value.SortOrder, IsActive: value.IsActive}
}
func productTypeData(value ProductType) ProductTypeData {
	return ProductTypeData{ID: value.ID, Code: value.Code, Name: value.Name, SortOrder: value.SortOrder}
}
func categoryData(value ProductCategory) CategoryData {
	return CategoryData{ID: value.ID, Name: value.Name, SortOrder: value.SortOrder, IsActive: value.IsActive}
}
func orderTypeData(value OrderType) OrderTypeData {
	return OrderTypeData{ID: value.ID, Code: value.Code, Name: value.Name, IconKey: value.IconKey, SortOrder: value.SortOrder, IsActive: value.IsActive}
}
func productData(value Product) ProductData {
	variations := make([]ProductVariantData, 0, len(value.Variants))
	for _, variant := range value.Variants {
		variations = append(variations, variantData(variant))
	}
	return ProductData{ID: value.ID, Name: value.Name, Price: value.Price, SellingPrice: value.Price,
		OnlineSellingPrice: value.OnlineSellingPrice, CostPrice: value.CostPrice, OnlineCostPrice: value.OnlineCostPrice,
		StockStatus: value.StockStatus, TrackStock: value.TrackStock, StockQuantity: value.StockQuantity, LowStockThreshold: value.LowStockThreshold, EffectiveStockStatus: value.EffectiveStockStatus(), Description: value.Description, ImageURL: value.ImageURL,
		IsHighlighted: value.IsHighlighted, SortOrder: value.SortOrder, IsActive: value.IsActive,
		Category: categoryData(value.Category), ProductTypeCode: value.Category.ProductType.Code, Variations: variations}
}
func productsData(values []Product) []ProductData {
	output := make([]ProductData, 0, len(values))
	for _, value := range values {
		output = append(output, productData(value))
	}
	return output
}

func variantMutations(values []ProductVariantRequest) []ProductVariantMutation {
	output := make([]ProductVariantMutation, 0, len(values))
	for _, value := range values {
		output = append(output, ProductVariantMutation{Name: value.Name, Price: value.SellingPrice,
			OnlineSellingPrice: value.OnlineSellingPrice, CostPrice: value.CostPrice, OnlineCostPrice: value.OnlineCostPrice,
			StockStatus: value.StockStatus, SortOrder: value.SortOrder, IsActive: true})
	}
	return output
}

func variantData(value ProductVariant) ProductVariantData {
	return ProductVariantData{ID: value.ID, Name: value.Name, SellingPrice: value.Price,
		OnlineSellingPrice: value.OnlineSellingPrice, CostPrice: value.CostPrice, OnlineCostPrice: value.OnlineCostPrice,
		StockStatus: value.StockStatus, SortOrder: value.SortOrder, IsActive: value.IsActive}
}

func catalogStatusError(err error) *sharederrors.StatusError {
	switch {
	case errors.Is(err, ErrInsufficientStock):
		return sharederrors.New(409, "STOK_TIDAK_CUKUP", "Stok tidak cukup atau jumlah di luar batas.", nil)
	case errors.Is(err, ErrForbidden):
		return sharederrors.Standard(403, nil)
	case errors.Is(err, ErrNotFound):
		return sharederrors.Standard(404, nil)
	case errors.Is(err, ErrConflict):
		return sharederrors.Standard(409, nil)
	case errors.Is(err, ErrCategoryInUse):
		return sharederrors.New(409, "KATEGORI_MASIH_DIGUNAKAN", "Kategori masih digunakan oleh produk aktif.", nil)
	case errors.Is(err, ErrImageTooLarge):
		return sharederrors.New(413, "GAMBAR_TERLALU_BESAR", "Ukuran gambar maksimal 3 MiB.", nil)
	case errors.Is(err, ErrInvalidImage):
		return sharederrors.New(422, "GAMBAR_TIDAK_VALID", "Gambar harus berupa PNG atau JPEG yang valid.", nil)
	case errors.Is(err, ErrValidation), errors.Is(err, ErrCategoryMismatch):
		return sharederrors.Standard(422, nil)
	case errors.Is(err, ErrStorageDisabled):
		return sharederrors.New(503, "PENYIMPANAN_GAMBAR_BELUM_DIKONFIGURASI", "Penyimpanan gambar belum dikonfigurasi.", nil)
	case errors.Is(err, ErrStorage):
		return sharederrors.New(502, "PENYIMPANAN_GAMBAR_BERMASALAH", "Penyimpanan gambar sedang bermasalah.", nil)
	case errors.Is(err, ErrUserWithoutOutlet):
		return sharederrors.Standard(403, nil)
	default:
		return sharederrors.Standard(500, nil)
	}
}
