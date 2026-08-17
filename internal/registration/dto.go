package registration

// RegisterRequest adalah payload gabungan tiga langkah form registrasi.
type RegisterRequest struct {
	BusinessName         string  `json:"business_name" validate:"required,min=2,max=120" minLength:"2" maxLength:"120" example:"Warung Dikasa"`
	BusinessAddress      string  `json:"business_address" validate:"required,min=5,max=1000" minLength:"5" maxLength:"1000" example:"Jl. Merdeka No. 10"`
	BusinessPhone        string  `json:"business_phone" validate:"required,min=8,max=30" minLength:"8" maxLength:"30" example:"081234567890"`
	BusinessEmail        *string `json:"business_email,omitempty" validate:"omitempty,email,max=254" format:"email" maxLength:"254" example:"owner@warung.example"`
	OwnerName            string  `json:"owner_name" validate:"required,min=2,max=200" minLength:"2" maxLength:"200" example:"Budi Santoso"`
	OwnerDateOfBirth     string  `json:"owner_date_of_birth" validate:"required,datetime=2006-01-02" format:"date" example:"1990-01-02"`
	OwnerPhone           string  `json:"owner_phone" validate:"required,min=8,max=30" minLength:"8" maxLength:"30" example:"081298765432"`
	OwnerEmail           string  `json:"owner_email" validate:"required,email,max=254" format:"email" maxLength:"254" example:"budi@example.com"`
	Username             string  `json:"username" validate:"required,alphanum,min=3,max=50" pattern:"^[A-Za-z0-9]+$" minLength:"3" maxLength:"50" example:"budisantoso"`
	Password             string  `json:"password" validate:"required,min=8,max=72" minLength:"8" maxLength:"72"`
	PasswordConfirmation string  `json:"password_confirmation" validate:"required,eqfield=Password" minLength:"8" maxLength:"72"`
	VerificationEmail    string  `json:"verification_email" validate:"required,email,max=254" format:"email" maxLength:"254" example:"budi@example.com"`
}

// RegisterInput membungkus body endpoint register untuk Huma.
type RegisterInput struct{ Body RegisterRequest }

// ConfirmOTPRequest adalah payload konfirmasi OTP email.
type ConfirmOTPRequest struct {
	Email string `json:"email" validate:"required,email,max=254" format:"email" maxLength:"254" example:"budi@example.com"`
	OTP   string `json:"otp" validate:"required,numeric,len=5" pattern:"^[0-9]{5}$" minLength:"5" maxLength:"5" example:"12345"`
}

// ConfirmOTPInput membungkus body endpoint konfirmasi OTP.
type ConfirmOTPInput struct{ Body ConfirmOTPRequest }

// ResendOTPRequest adalah payload pengiriman ulang OTP.
type ResendOTPRequest struct {
	Email string `json:"email" validate:"required,email,max=254" format:"email" maxLength:"254" example:"budi@example.com"`
}

// ResendOTPInput membungkus body endpoint kirim ulang OTP.
type ResendOTPInput struct{ Body ResendOTPRequest }

// OTPResponse adalah envelope respons register dan resend.
type OTPResponse struct {
	Data OTPResponseData `json:"data"`
}

// OTPResponseData memuat email tujuan dan timer dalam detik.
type OTPResponseData struct {
	Email             string `json:"email" format:"email"`
	OTPExpiresIn      int    `json:"otp_expires_in" example:"300"`
	ResendAvailableIn int    `json:"resend_available_in" example:"60"`
}

// RegisterOutput membungkus respons 201 endpoint register.
type RegisterOutput struct {
	Status int
	Body   OTPResponse
}

// ResendOTPOutput membungkus respons sukses kirim ulang OTP.
type ResendOTPOutput struct{ Body OTPResponse }

// ConfirmOTPResponse adalah envelope status verifikasi.
type ConfirmOTPResponse struct {
	Data ConfirmOTPResponseData `json:"data"`
}

// ConfirmOTPResponseData menyatakan bahwa aktivasi akun telah selesai.
type ConfirmOTPResponseData struct {
	Verified bool `json:"verified" example:"true"`
}

// ConfirmOTPOutput membungkus respons sukses konfirmasi OTP.
type ConfirmOTPOutput struct{ Body ConfirmOTPResponse }
