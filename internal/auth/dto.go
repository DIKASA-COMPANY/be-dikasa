package auth

// Session holds a provider-issued access token and an optional Firebase refresh token.
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

// LoginResponse is the Firebase session envelope returned after credential validation.
type LoginResponse struct {
	Data LoginResponseData `json:"data"`
}

// LoginResponseData contains the Bearer session issued by Firebase Authentication.
type LoginResponseData struct {
	AccessToken  string `json:"access_token" doc:"Firebase ID token untuk Authorization Bearer"`
	RefreshToken string `json:"refresh_token" doc:"Firebase refresh token"`
	TokenType    string `json:"token_type" enum:"Bearer" example:"Bearer"`
	ExpiresIn    int    `json:"expires_in" example:"3600"`
	FirebaseUID  string `json:"firebase_uid" doc:"UUID user PostgreSQL yang dipakai sebagai Firebase UID"`
}

// LoginOutput membungkus respons sukses endpoint login untuk Huma.
type LoginOutput struct {
	Body LoginResponse
}
