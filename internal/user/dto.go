package user

import "github.com/danielgtaylor/huma/v2"

// Profile adalah data user yang aman dikirim ke client.
type Profile struct {
	ID          string  `json:"id"`
	RoleID      string  `json:"role_id"`
	GenderID    *string `json:"gender_id,omitempty"`
	OutletID    *string `json:"outlet_id,omitempty"`
	Username    string  `json:"username"`
	PhoneNumber string  `json:"phone_number"`
	Email       *string `json:"email,omitempty"`
	FirstName   string  `json:"first_name"`
	LastName    string  `json:"last_name"`
	IsActive    bool    `json:"is_active"`
	AvatarURL   *string `json:"avatar_url,omitempty" format:"uri"`
}

// ProfileResponse adalah envelope response profil user.
type ProfileResponse struct {
	Data Profile `json:"data"`
}

// ProfileOutput membungkus response profil untuk Huma.
type ProfileOutput struct {
	Body ProfileResponse
}

// ProfileForm adalah field multipart opsional untuk edit profil sendiri.
type ProfileForm struct {
	FirstName    string        `form:"first_name" required:"false" maxLength:"100"`
	LastName     string        `form:"last_name" required:"false" maxLength:"100"`
	PhoneNumber  string        `form:"phone_number" required:"false" minLength:"8" maxLength:"30"`
	Email        string        `form:"email" required:"false" maxLength:"254"`
	RemoveAvatar bool          `form:"remove_avatar" required:"false"`
	Avatar       huma.FormFile `form:"avatar" required:"false" contentType:"image/png,image/jpeg"`
}

// UpdateProfileInput adalah multipart edit profil user terautentikasi.
type UpdateProfileInput struct {
	RawBody huma.MultipartFormFiles[ProfileForm]
}

// ChangePINRequest adalah payload perubahan PIN enam digit.
type ChangePINRequest struct {
	CurrentPIN         string `json:"current_pin,omitempty" validate:"omitempty,numeric,len=6" pattern:"^[0-9]{6}$" minLength:"6" maxLength:"6"`
	NewPIN             string `json:"new_pin" validate:"required,numeric,len=6" pattern:"^[0-9]{6}$" minLength:"6" maxLength:"6"`
	NewPINConfirmation string `json:"new_pin_confirmation" validate:"required,eqfield=NewPIN" pattern:"^[0-9]{6}$" minLength:"6" maxLength:"6"`
}

// ChangePINInput membungkus payload perubahan PIN.
type ChangePINInput struct{ Body ChangePINRequest }

// ChangePasswordRequest adalah payload perubahan password akun.
type ChangePasswordRequest struct {
	CurrentPassword         string `json:"current_password" validate:"required,min=8,max=72" minLength:"8" maxLength:"72"`
	NewPassword             string `json:"new_password" validate:"required,min=8,max=72" minLength:"8" maxLength:"72"`
	NewPasswordConfirmation string `json:"new_password_confirmation" validate:"required,eqfield=NewPassword" minLength:"8" maxLength:"72"`
}

// ChangePasswordInput membungkus payload perubahan password.
type ChangePasswordInput struct{ Body ChangePasswordRequest }

// NoContentOutput membungkus response 204 pengaturan akun.
type NoContentOutput struct{ Status int }
