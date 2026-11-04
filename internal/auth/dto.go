package auth

// Session holds a Supabase Auth access and refresh session.
type Session struct {
	AccessToken    string
	RefreshToken   string
	ExpiresIn      int
	SupabaseUserID string
}

// LoginRequest adalah payload login username, nomor HP, atau email.
type LoginRequest struct {
	Identifier string `json:"identifier" validate:"required,min=3,max=254" minLength:"3" maxLength:"254" doc:"Username, nomor HP, atau email user PostgreSQL" example:"kasir"`
	Password   string `json:"password" validate:"required,min=8,max=128" minLength:"8" maxLength:"128" doc:"Password user PostgreSQL"`
}

// LoginInput membungkus body endpoint login untuk Huma.
type LoginInput struct {
	Body LoginRequest
}

// LoginResponse is the Supabase session envelope returned after credential validation.
type LoginResponse struct {
	Data LoginResponseData `json:"data"`
}

// LoginResponseData contains the Bearer session issued by Supabase Auth.
type LoginResponseData struct {
	AccessToken    string `json:"access_token" doc:"Supabase access token untuk Authorization Bearer"`
	RefreshToken   string `json:"refresh_token" doc:"Supabase refresh token"`
	TokenType      string `json:"token_type" enum:"Bearer" example:"Bearer"`
	ExpiresIn      int    `json:"expires_in" example:"3600"`
	SupabaseUserID string `json:"supabase_user_id" doc:"UUID identitas Supabase Auth yang terikat ke user PostgreSQL"`
}

// LoginOutput membungkus respons sukses endpoint login untuk Huma.
type LoginOutput struct {
	Body LoginResponse
}
