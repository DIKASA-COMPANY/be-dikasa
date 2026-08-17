package user

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
}

// ProfileResponse adalah envelope response profil user.
type ProfileResponse struct {
	Data Profile `json:"data"`
}

// ProfileOutput membungkus response profil untuk Huma.
type ProfileOutput struct {
	Body ProfileResponse
}
