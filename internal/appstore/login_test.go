package appstore

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	ipatool "github.com/majd/ipatool/v2/pkg/appstore"
)

// loginResponse mirrors the shape of the http.Result ipatool attaches to its
// login errors.
type loginResponse struct {
	StatusCode int
	Data       struct {
		FailureType     string
		CustomerMessage string
	}
}

func failedLogin(failureType, message string) error {
	var res loginResponse
	res.StatusCode = 200
	res.Data.FailureType = failureType
	res.Data.CustomerMessage = message

	return ipatool.NewErrorWithMetadata(errors.New(message), res)
}

func TestLoginError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		is       error
		contains string
	}{
		{
			name: "2FA code required",
			err:  fmt.Errorf("wrapped: %w", ipatool.ErrAuthCodeRequired),
			is:   ErrAuthCodeRequired,
		},
		{
			name:     "wrong password",
			err:      failedLogin(failureInvalidCredentials, "Your Apple ID or password was entered incorrectly."),
			is:       ErrInvalidCredentials,
			contains: "entered incorrectly",
		},
		{
			name:     "other Apple failure",
			err:      failedLogin("2059", "Temporarily unavailable"),
			contains: "login: Temporarily unavailable",
		},
		{
			name:     "transport error",
			err:      errors.New("sign-in request failed: connection reset"),
			contains: "login: sign-in request failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := loginError(tt.err)
			if tt.is != nil && !errors.Is(got, tt.is) {
				t.Fatalf("loginError() = %v, want it to wrap %v", got, tt.is)
			}
			if tt.is == nil && (errors.Is(got, ErrInvalidCredentials) || errors.Is(got, ErrAuthCodeRequired)) {
				t.Fatalf("loginError() = %v, classified as a credential error", got)
			}
			if tt.contains != "" && !strings.Contains(got.Error(), tt.contains) {
				t.Fatalf("loginError() = %q, want it to contain %q", got, tt.contains)
			}
		})
	}
}

func TestDiscardKeychain(t *testing.T) {
	var k discardKeychain
	if err := k.Set("account", []byte("{}")); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if _, err := k.Get("account"); err == nil {
		t.Fatal("Get returned an account; nothing should be stored")
	}
}
