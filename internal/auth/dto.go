package auth

// Session menyimpan Firebase ID token dan refresh token setelah login berhasil.
type Session struct {
	IDToken      string
	RefreshToken string
	ExpiresIn    int
	FirebaseUID  string
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

// LoginResponse adalah envelope sesi Firebase setelah autentikasi berhasil.
type LoginResponse struct {
	Data LoginResponseData `json:"data"`
}

// LoginResponseData memuat Firebase ID token yang digunakan sebagai Bearer token.
type LoginResponseData struct {
	AccessToken  string `json:"access_token" doc:"Firebase ID token untuk Authorization Bearer"`
	RefreshToken string `json:"refresh_token" doc:"Firebase refresh token untuk memperbarui sesi"`
	TokenType    string `json:"token_type" enum:"Bearer" example:"Bearer"`
	ExpiresIn    int    `json:"expires_in" example:"3600"`
	FirebaseUID  string `json:"firebase_uid" doc:"Sama dengan UUID user PostgreSQL"`
}

// LoginOutput membungkus respons sukses endpoint login untuk Huma.
type LoginOutput struct {
	Body LoginResponse
}
