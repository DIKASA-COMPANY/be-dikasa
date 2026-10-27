package user

import (
	"context"
	"errors"
	"net/http"

	sharederrors "be-dikasa/internal/shared/errors"
	sharedvalidator "be-dikasa/internal/shared/validator"
)

// Handler melayani endpoint user melalui Huma.
type Handler struct {
	service   *Service
	validator *sharedvalidator.Validator
}

// NewHandler membuat handler user.
func NewHandler(service *Service, validators ...*sharedvalidator.Validator) *Handler {
	validator := sharedvalidator.New()
	if len(validators) > 0 && validators[0] != nil {
		validator = validators[0]
	}
	return &Handler{service: service, validator: validator}
}

// Me mengembalikan profil user yang sedang terautentikasi.
func (handler *Handler) Me(ctx context.Context, _ *struct{}) (*ProfileOutput, error) {
	userID, ok := AuthenticatedUserIDFromContext(ctx)
	if !ok {
		return nil, sharederrors.Standard(http.StatusUnauthorized, nil)
	}
	account, err := handler.service.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, sharederrors.Standard(http.StatusNotFound, nil)
		}
		return nil, sharederrors.Standard(http.StatusInternalServerError, nil)
	}
	data := profileData(account)
	data.CanChangePassword = handler.service.passwordChangesAllowed
	return &ProfileOutput{Body: ProfileResponse{Data: data}}, nil
}

// UpdateProfile memperbarui field multipart profil user terautentikasi.
func (handler *Handler) UpdateProfile(ctx context.Context, input *UpdateProfileInput) (*ProfileOutput, error) {
	userID, ok := AuthenticatedUserIDFromContext(ctx)
	if !ok {
		return nil, sharederrors.Standard(http.StatusUnauthorized, nil)
	}
	form := input.RawBody.Data()
	fields := make(map[string]bool)
	if input.RawBody.Form != nil {
		for key := range input.RawBody.Form.Value {
			fields[key] = true
		}
	}
	mutation := ProfileMutation{DateOfBirth: form.DateOfBirth, GenderID: form.GenderID, PersonalAddress: form.PersonalAddress, FirstName: form.FirstName, LastName: form.LastName, PhoneNumber: form.PhoneNumber, Email: form.Email, RemoveAvatar: form.RemoveAvatar, Fields: fields}
	if form.Avatar.IsSet {
		mutation.Avatar = &AvatarUpload{Reader: form.Avatar.File, Size: form.Avatar.Size, ContentType: form.Avatar.ContentType, Filename: form.Avatar.Filename}
	}
	account, err := handler.service.UpdateProfile(ctx, userID, mutation)
	if err != nil {
		return nil, settingsStatusError(err)
	}
	data := profileData(account)
	data.CanChangePassword = handler.service.passwordChangesAllowed
	return &ProfileOutput{Body: ProfileResponse{Data: data}}, nil
}

// ChangePIN memvalidasi konfirmasi dan mengganti PIN user terautentikasi.
func (handler *Handler) ChangePIN(ctx context.Context, input *ChangePINInput) (*NoContentOutput, error) {
	userID, ok := AuthenticatedUserIDFromContext(ctx)
	if !ok {
		return nil, sharederrors.Standard(http.StatusUnauthorized, nil)
	}
	if details := handler.validator.Validate(input.Body); len(details) > 0 {
		return nil, sharederrors.New(422, "VALIDASI_GAGAL", "Data yang dikirim tidak memenuhi ketentuan.", details)
	}
	if err := handler.service.ChangePIN(ctx, userID, input.Body.CurrentPIN, input.Body.NewPIN); err != nil {
		return nil, settingsStatusError(err)
	}
	return &NoContentOutput{Status: http.StatusNoContent}, nil
}

// ChangePassword memvalidasi konfirmasi dan mengganti password user terautentikasi.
func (handler *Handler) ChangePassword(ctx context.Context, input *ChangePasswordInput) (*NoContentOutput, error) {
	userID, ok := AuthenticatedUserIDFromContext(ctx)
	if !ok {
		return nil, sharederrors.Standard(http.StatusUnauthorized, nil)
	}
	if details := handler.validator.Validate(input.Body); len(details) > 0 {
		return nil, sharederrors.New(422, "VALIDASI_GAGAL", "Data yang dikirim tidak memenuhi ketentuan.", details)
	}
	if err := handler.service.ChangePassword(ctx, userID, input.Body.CurrentPassword, input.Body.NewPassword); err != nil {
		return nil, settingsStatusError(err)
	}
	return &NoContentOutput{Status: http.StatusNoContent}, nil
}

func profileData(account *User) Profile {
	p := Profile{Role: account.Role.Name, PersonalAddress: account.PersonalAddress, HasPIN: account.PINHash != nil, ID: account.ID, RoleID: account.RoleID, GenderID: account.GenderID, OutletID: account.OutletID, Username: account.Username,
		PhoneNumber: account.PhoneNumber, Email: account.Email, FirstName: account.FirstName, LastName: account.LastName, IsActive: account.IsActive, AvatarURL: account.AvatarURL}
	if account.Outlet != nil {
		p.OutletName = account.Outlet.Name
	}
	if account.Gender != nil {
		v := account.Gender.Name
		p.GenderName = &v
	}
	if account.DateOfBirth != nil {
		v := account.DateOfBirth.Format("2006-01-02")
		p.DateOfBirth = &v
	}
	return p
}

// settingsStatusError mencegah detail hash, database, dan storage bocor ke client.
func settingsStatusError(err error) *sharederrors.StatusError {
	switch {
	case errors.Is(err, ErrPasswordChangesDisabled):
		return sharederrors.New(503, "PASSWORD_PROVIDER_TIDAK_DIDUKUNG", "Perubahan password tidak tersedia untuk penyedia autentikasi ini.", nil)
	case errors.Is(err, ErrNotFound):
		return sharederrors.Standard(404, nil)
	case errors.Is(err, ErrConflict):
		return sharederrors.Standard(409, map[string]string{"profile": "Nomor HP atau email sudah digunakan."})
	case errors.Is(err, ErrInvalidCurrentPIN):
		return sharederrors.New(422, "PIN_LAMA_TIDAK_VALID", "PIN lama tidak valid.", nil)
	case errors.Is(err, ErrInvalidCurrentPassword):
		return sharederrors.New(422, "PASSWORD_LAMA_TIDAK_VALID", "Password lama tidak valid.", nil)
	case errors.Is(err, ErrCredentialReuse):
		return sharederrors.New(422, "KREDENSIAL_BARU_SAMA", "Kredensial baru harus berbeda dari kredensial lama.", nil)
	case errors.Is(err, ErrInvalidProfile), errors.Is(err, ErrInvalidPIN), errors.Is(err, ErrInvalidPassword):
		return sharederrors.Standard(422, nil)
	case errors.Is(err, ErrAvatarTooLarge):
		return sharederrors.New(413, "FOTO_PROFIL_TERLALU_BESAR", "Ukuran foto profil maksimal 3 MiB.", nil)
	case errors.Is(err, ErrInvalidAvatar):
		return sharederrors.New(422, "FOTO_PROFIL_TIDAK_VALID", "Foto profil harus berupa PNG atau JPEG yang valid.", nil)
	case errors.Is(err, ErrAvatarStorageDisabled):
		return sharederrors.New(503, "PENYIMPANAN_GAMBAR_BELUM_DIKONFIGURASI", "Penyimpanan gambar belum dikonfigurasi.", nil)
	case errors.Is(err, ErrAvatarStorage):
		return sharederrors.New(502, "PENYIMPANAN_GAMBAR_BERMASALAH", "Penyimpanan gambar sedang bermasalah.", nil)
	default:
		return sharederrors.Standard(500, nil)
	}
}

// GenderChoice exposes only reference identifiers and labels.
type GenderChoice struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// GenderOutput wraps gender reference options.
type GenderOutput struct {
	Body struct {
		Data []GenderChoice `json:"data"`
	}
}

// Genders returns reference choices for the authenticated account profile.
func (h *Handler) Genders(ctx context.Context, _ *struct{}) (*GenderOutput, error) {
	if _, ok := AuthenticatedUserIDFromContext(ctx); !ok {
		return nil, sharederrors.Standard(401, nil)
	}
	choices, e := h.service.ListGenders(ctx)
	if e != nil {
		return nil, settingsStatusError(e)
	}
	out := &GenderOutput{}
	out.Body.Data = []GenderChoice{}
	for _, g := range choices {
		out.Body.Data = append(out.Body.Data, GenderChoice{g.ID, g.Name})
	}
	return out, nil
}
