package user

import "github.com/danielgtaylor/huma/v2"

// Profile adalah data user yang aman dikirim ke client.
type Profile struct {
	Role              string  `json:"role"`
	OutletName        string  `json:"outlet_name"`
	GenderName        *string `json:"gender_name"`
	DateOfBirth       *string `json:"date_of_birth"`
	PersonalAddress   *string `json:"personal_address"`
	HasPIN            bool    `json:"has_pin"`
	CanChangePassword bool    `json:"can_change_password"`
	ID                string  `json:"id"`
	RoleID            string  `json:"role_id"`
	GenderID          *string `json:"gender_id,omitempty"`
	OutletID          *string `json:"outlet_id,omitempty"`
	Username          string  `json:"username"`
	PhoneNumber       string  `json:"phone_number"`
	Email             *string `json:"email,omitempty"`
	FirstName         string  `json:"first_name"`
	LastName          string  `json:"last_name"`
	IsActive          bool    `json:"is_active"`
	AvatarURL         *string `json:"avatar_url,omitempty" format:"uri"`
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
	DateOfBirth     string        `form:"date_of_birth" required:"false"`
	GenderID        string        `form:"gender_id" required:"false"`
	PersonalAddress string        `form:"personal_address" required:"false" maxLength:"1000"`
	FirstName       string        `form:"first_name" required:"false" maxLength:"100"`
	LastName        string        `form:"last_name" required:"false" maxLength:"100"`
	PhoneNumber     string        `form:"phone_number" required:"false" minLength:"8" maxLength:"30"`
	Email           string        `form:"email" required:"false" maxLength:"254"`
	RemoveAvatar    bool          `form:"remove_avatar" required:"false"`
	Avatar          huma.FormFile `form:"avatar" required:"false" contentType:"image/png,image/jpeg"`
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
	CurrentPassword         string `json:"current_password" validate:"required,max=72" minLength:"1" maxLength:"72" doc:"Panjang 8–72 byte UTF-8; divalidasi service."`
	NewPassword             string `json:"new_password" validate:"required,max=72" minLength:"1" maxLength:"72" doc:"Panjang 8–72 byte UTF-8; divalidasi service."`
	NewPasswordConfirmation string `json:"new_password_confirmation" validate:"required,eqfield=NewPassword" minLength:"1" maxLength:"72"`
}

// ChangePasswordInput membungkus payload perubahan password.
type ChangePasswordInput struct{ Body ChangePasswordRequest }

// NoContentOutput membungkus response 204 pengaturan akun.
type NoContentOutput struct{ Status int }
