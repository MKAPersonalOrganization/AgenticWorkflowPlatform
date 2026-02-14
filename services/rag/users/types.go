package users

// UserInfo represents user information returned by the API.
type UserInfo struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Role     string `json:"role"`
}

// CreateUserRequest represents the payload to create a new user.
type CreateUserRequest struct {
	Username string  `json:"username"`
	Email    string  `json:"email"`
	Password string  `json:"password"`
	Role     *string `json:"role,omitempty"`
}

// CreateUserResponse represents the create user response.
type CreateUserResponse struct {
	User      UserInfo `json:"user"`
	CreatedAt string   `json:"created_at"`
}

// ListUsersResponse represents a paginated list of users.
type ListUsersResponse struct {
	Users      []UserInfo `json:"users"`
	Total      int        `json:"total"`
	Page       int        `json:"page"`
	PageSize   int        `json:"page_size"`
	TotalPages int        `json:"total_pages"`
}

// ListUsersParams represents query parameters for listing users.
type ListUsersParams struct {
	Page     *int
	PageSize *int
	Role     *string
}
