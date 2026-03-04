package auth

import (
	"context"
	"testing"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

func newTestService(t *testing.T) *Service {
	t.Helper()

	logger, _ := zap.NewDevelopment()

	hash, err := bcrypt.GenerateFromPassword([]byte("testpassword"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}

	cfg := Config{
		Local: LocalAuthConfig{
			Enabled: true,
			Admin: User{
				Username:     "admin",
				PasswordHash: string(hash),
			},
		},
		JWTSecret: "test-secret-key-at-least-32-bytes!",
		TokenTTL:  0,
	}

	svc, err := NewService(cfg, nil, logger)
	if err != nil {
		t.Fatalf("failed to create auth service: %v", err)
	}
	return svc
}

func TestLogin_Success(t *testing.T) {
	svc := newTestService(t)
	resp, err := svc.Login(context.Background(), LoginRequest{
		Username: "admin",
		Password: "testpassword",
	})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	if resp.Token == "" {
		t.Error("expected non-empty token")
	}
	if resp.User == nil {
		t.Error("expected user in response")
	}
	if resp.User.Username != "admin" {
		t.Errorf("expected admin username, got %s", resp.User.Username)
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	svc := newTestService(t)
	_, err := svc.Login(context.Background(), LoginRequest{
		Username: "admin",
		Password: "wrongpassword",
	})
	if err == nil {
		t.Error("expected error for wrong password")
	}
}

func TestLogin_UnknownUser(t *testing.T) {
	svc := newTestService(t)
	_, err := svc.Login(context.Background(), LoginRequest{
		Username: "nobody",
		Password: "whatever",
	})
	if err == nil {
		t.Error("expected error for unknown user")
	}
}

func TestValidateToken(t *testing.T) {
	svc := newTestService(t)

	resp, err := svc.Login(context.Background(), LoginRequest{
		Username: "admin",
		Password: "testpassword",
	})
	if err != nil {
		t.Fatal(err)
	}

	claims, err := svc.ValidateToken(context.Background(), resp.Token)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}
	if claims.Username != "admin" {
		t.Errorf("expected admin, got %s", claims.Username)
	}
}

func TestValidateToken_BearerPrefix(t *testing.T) {
	svc := newTestService(t)

	resp, _ := svc.Login(context.Background(), LoginRequest{
		Username: "admin",
		Password: "testpassword",
	})

	claims, err := svc.ValidateToken(context.Background(), "Bearer "+resp.Token)
	if err != nil {
		t.Fatalf("ValidateToken with Bearer prefix failed: %v", err)
	}
	if claims.Username != "admin" {
		t.Errorf("expected admin, got %s", claims.Username)
	}
}

func TestValidateToken_InvalidToken(t *testing.T) {
	svc := newTestService(t)
	_, err := svc.ValidateToken(context.Background(), "not-a-valid-token")
	if err == nil {
		t.Error("expected error for invalid token")
	}
}

func TestCreateUser(t *testing.T) {
	svc := newTestService(t)

	user, err := svc.CreateUser(context.Background(), CreateUserRequest{
		Username: "operator1",
		Password: "secure-pass-123",
		Roles:    []string{"operator"},
	})
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	if user.Username != "operator1" {
		t.Errorf("unexpected username: %s", user.Username)
	}
	if user.PasswordHash != "" {
		t.Error("password hash should be sanitized from response")
	}
	if len(user.Roles) != 1 || user.Roles[0] != "operator" {
		t.Error("roles not set correctly")
	}
}

func TestCreateUser_Duplicate(t *testing.T) {
	svc := newTestService(t)
	_, err := svc.CreateUser(context.Background(), CreateUserRequest{
		Username: "admin",
		Password: "newpass",
	})
	if err == nil {
		t.Error("expected error for duplicate user")
	}
}

func TestLogout(t *testing.T) {
	svc := newTestService(t)

	resp, _ := svc.Login(context.Background(), LoginRequest{
		Username: "admin",
		Password: "testpassword",
	})

	if err := svc.Logout(context.Background(), resp.Token); err != nil {
		t.Errorf("logout failed: %v", err)
	}
}

func TestClaims_HasRole(t *testing.T) {
	c := &Claims{Roles: []string{"operator"}}
	if c.HasRole("admin") {
		t.Error("operator should not have admin role")
	}
	if !c.HasRole("operator") {
		t.Error("operator should have operator role")
	}

	adminClaims := &Claims{Roles: []string{"admin"}}
	if !adminClaims.HasRole("operator") {
		t.Error("admin should pass any role check")
	}
	if !adminClaims.HasRole("viewer") {
		t.Error("admin should pass viewer role check")
	}
}

func TestListUsers(t *testing.T) {
	svc := newTestService(t)
	users, err := svc.ListUsers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(users) == 0 {
		t.Error("expected at least admin user")
	}
}

func TestDeleteUser(t *testing.T) {
	svc := newTestService(t)

	user, _ := svc.CreateUser(context.Background(), CreateUserRequest{
		Username: "todelete",
		Password: "pass123",
	})

	if err := svc.DeleteUser(context.Background(), user.ID); err != nil {
		t.Fatalf("DeleteUser failed: %v", err)
	}

	users, _ := svc.ListUsers(context.Background())
	for _, u := range users {
		if u.Username == "todelete" {
			t.Error("user should have been deleted")
		}
	}
}

func TestAuditLog(t *testing.T) {
	svc := newTestService(t)

	// Trigger audit entries via login
	_, _ = svc.Login(context.Background(), LoginRequest{Username: "admin", Password: "testpassword"})
	_, _ = svc.Login(context.Background(), LoginRequest{Username: "admin", Password: "wrongpass"})

	logs, err := svc.GetAuditLog(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) < 2 {
		t.Errorf("expected at least 2 audit entries, got %d", len(logs))
	}
}
