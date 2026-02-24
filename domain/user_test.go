package domain

import (
	"testing"
	"time"

	client "github.com/ory/client-go"
)

func TestNewUserFromIdentity(t *testing.T) {
	t.Parallel()
	now := time.Now()

	tests := []struct {
		name     string
		identity *client.Identity
		wantUser *User
		wantErr  bool
	}{
		{
			name:     "nil identity",
			identity: nil,
			wantUser: nil,
			wantErr:  true,
		},
		{
			name: "identity with email only",
			identity: &client.Identity{
				Id:        "user-123",
				CreatedAt: &now,
				UpdatedAt: &now,
				Traits: map[string]any{
					"email": "test@example.com",
				},
			},
			wantUser: &User{
				ID:        "user-123",
				Email:     "test@example.com",
				CreatedAt: now,
				UpdatedAt: now,
				Traits: IdentityTraits{
					Email: "test@example.com",
				},
			},
			wantErr: false,
		},
		{
			name: "identity with full name",
			identity: &client.Identity{
				Id:        "user-456",
				CreatedAt: &now,
				UpdatedAt: &now,
				Traits: map[string]any{
					"email": "john@example.com",
					"name": map[string]any{
						"first": "John",
						"last":  "Doe",
					},
				},
			},
			wantUser: &User{
				ID:        "user-456",
				Email:     "john@example.com",
				FirstName: "John",
				LastName:  "Doe",
				CreatedAt: now,
				UpdatedAt: now,
				Traits: IdentityTraits{
					Email: "john@example.com",
					Name: NameTraits{
						First: "John",
						Last:  "Doe",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "identity with first name only",
			identity: &client.Identity{
				Id:        "user-789",
				CreatedAt: &now,
				UpdatedAt: &now,
				Traits: map[string]any{
					"email": "jane@example.com",
					"name": map[string]any{
						"first": "Jane",
					},
				},
			},
			wantUser: &User{
				ID:        "user-789",
				Email:     "jane@example.com",
				FirstName: "Jane",
				CreatedAt: now,
				UpdatedAt: now,
				Traits: IdentityTraits{
					Email: "jane@example.com",
					Name: NameTraits{
						First: "Jane",
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := NewUserFromIdentity(tt.identity)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewUserFromIdentity() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if tt.wantUser == nil && got != nil {
				t.Errorf("NewUserFromIdentity() expected nil, got %+v", got)

				return
			}

			if tt.wantUser != nil && got == nil {
				t.Errorf("NewUserFromIdentity() expected user, got nil")

				return
			}

			if got != nil {
				if got.ID != tt.wantUser.ID {
					t.Errorf("NewUserFromIdentity() ID = %v, want %v", got.ID, tt.wantUser.ID)
				}
				if got.Email != tt.wantUser.Email {
					t.Errorf("NewUserFromIdentity() Email = %v, want %v", got.Email, tt.wantUser.Email)
				}
				if got.FirstName != tt.wantUser.FirstName {
					t.Errorf("NewUserFromIdentity() FirstName = %v, want %v", got.FirstName, tt.wantUser.FirstName)
				}
				if got.LastName != tt.wantUser.LastName {
					t.Errorf("NewUserFromIdentity() LastName = %v, want %v", got.LastName, tt.wantUser.LastName)
				}
			}
		})
	}
}

func TestNewUserFromSession(t *testing.T) {
	t.Parallel()
	now := time.Now()

	tests := []struct {
		name    string
		session *client.Session
		wantNil bool
	}{
		{
			name:    "nil session",
			session: nil,
			wantNil: true,
		},
		{
			name: "session with identity",
			session: &client.Session{
				Identity: &client.Identity{
					Id:        "user-123",
					CreatedAt: &now,
					UpdatedAt: &now,
					Traits: map[string]any{
						"email": "test@example.com",
					},
				},
			},
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := NewUserFromSession(tt.session)
			if tt.wantNil {
				if err == nil {
					t.Errorf("NewUserFromSession() expected error, got nil")
					return
				}
				if got != nil {
					t.Errorf("NewUserFromSession() expected nil user, got %+v", got)
				}
			} else {
				if err != nil {
					t.Errorf("NewUserFromSession() unexpected error = %v", err)
					return
				}
				if got == nil {
					t.Errorf("NewUserFromSession() expected user, got nil")
				}
			}
		})
	}
}

func TestUser_GetFullName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		user User
		want string
	}{
		{
			name: "both first and last name",
			user: User{
				Email:     "john@example.com",
				FirstName: "John",
				LastName:  "Doe",
			},
			want: "John Doe",
		},
		{
			name: "first name only",
			user: User{
				Email:     "john@example.com",
				FirstName: "John",
			},
			want: "John",
		},
		{
			name: "last name only",
			user: User{
				Email:    "doe@example.com",
				LastName: "Doe",
			},
			want: "Doe",
		},
		{
			name: "no name",
			user: User{
				Email: "user@example.com",
			},
			want: "user@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.user.GetFullName(); got != tt.want {
				t.Errorf("User.GetFullName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUser_GetDisplayName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		user User
		want string
	}{
		{
			name: "with full name",
			user: User{
				Email:     "john@example.com",
				FirstName: "John",
				LastName:  "Doe",
			},
			want: "John Doe",
		},
		{
			name: "email only",
			user: User{
				Email: "user@example.com",
			},
			want: "user@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.user.GetDisplayName(); got != tt.want {
				t.Errorf("User.GetDisplayName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUser_HasName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		user User
		want bool
	}{
		{
			name: "has first name",
			user: User{FirstName: "John"},
			want: true,
		},
		{
			name: "has last name",
			user: User{LastName: "Doe"},
			want: true,
		},
		{
			name: "has both names",
			user: User{FirstName: "John", LastName: "Doe"},
			want: true,
		},
		{
			name: "has no name",
			user: User{Email: "user@example.com"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.user.HasName(); got != tt.want {
				t.Errorf("User.HasName() = %v, want %v", got, tt.want)
			}
		})
	}
}
