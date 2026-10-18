package appstore

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	ipatool "github.com/majd/ipatool/v2/pkg/appstore"
	"github.com/majd/ipatool/v2/pkg/util/operatingsystem"
)

// Login authenticates against the App Store and returns the account.
// Pass authCode when a prior call returned ErrAuthCodeRequired (2FA).
//
// Since August 2026 Apple only accepts sign-in requests signed with SAP
// (the X-Apple-ActionSignature header), so this uses ipatool's login,
// which signs them by running Apple's own FairPlay code in an emulator.
// The first sign-in downloads about 38 MB of Apple assets into the user
// cache directory, so it can take a minute.
func (c *Client) Login(email, password, authCode string) (*Account, error) {
	store := ipatool.NewAppStore(ipatool.Args{
		Keychain:        discardKeychain{},
		CookieJar:       c.jar,
		OperatingSystem: operatingsystem.New(),
		Machine:         hostMachine,
	})

	out, err := store.Login(ipatool.LoginInput{
		Email:    email,
		Password: password,
		AuthCode: authCode,
	})
	if err != nil {
		return nil, loginError(err)
	}

	acc := out.Account
	if acc.PasswordToken == "" || acc.DirectoryServicesID == "" {
		return nil, errors.New("login: no token in Apple's response")
	}

	return &Account{
		Email:               acc.Email,
		PasswordToken:       acc.PasswordToken,
		DirectoryServicesID: acc.DirectoryServicesID,
		StoreFront:          acc.StoreFront,
		Password:            password,
		Pod:                 acc.Pod,
		Name:                strings.TrimSpace(acc.Name),
	}, nil
}

// loginError maps ipatool's login errors onto the ones callers here check
// for. Apple reports a wrong password as failure type -5000; ipatool passes
// it on as Apple's customer message, with the response as metadata.
func loginError(err error) error {
	if errors.Is(err, ipatool.ErrAuthCodeRequired) {
		return ErrAuthCodeRequired
	}

	var appErr *ipatool.Error
	if errors.As(err, &appErr) {
		failureType, message := loginFailure(appErr.Metadata)
		if failureType == failureInvalidCredentials {
			if message != "" {
				return fmt.Errorf("%w: %s", ErrInvalidCredentials, message)
			}

			return ErrInvalidCredentials
		}
	}

	return fmt.Errorf("login: %w", err)
}

// loginFailure returns the failure type and customer message of the
// authenticate response in metadata, an http.Result of an unexported type.
func loginFailure(metadata any) (string, string) {
	data, err := json.Marshal(metadata)
	if err != nil {
		return "", ""
	}

	var res struct {
		Data struct {
			FailureType     string
			CustomerMessage string
		}
	}
	if json.Unmarshal(data, &res) != nil {
		return "", ""
	}

	return res.Data.FailureType, res.Data.CustomerMessage
}

// discardKeychain is the keychain given to ipatool, which saves the account
// there after signing in. Callers here store the returned account themselves.
type discardKeychain struct{}

func (discardKeychain) Get(string) ([]byte, error) { return nil, errors.New("not stored") }
func (discardKeychain) Set(string, []byte) error   { return nil }
func (discardKeychain) Remove(string) error        { return nil }
