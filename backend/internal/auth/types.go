package auth

type User struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	CSRFToken string `json:"csrfToken,omitempty"`
}

type ConfigResponse struct {
	SignupMode      string `json:"signupMode"`
	SignupAvailable bool   `json:"signupAvailable"`
}

type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type UserRow struct {
	ID           int64
	Username     string
	PasswordHash string
	CreatedAt    string
}

type SessionResult struct {
	User      User
	Token     string
	CSRFToken string
	ExpiresAt string
}
