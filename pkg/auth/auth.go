package auth

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net/http"
	"os"
	"strings"
)

var ErrUnauthorized = errors.New("proxy authentication required")

type Authenticator interface {
	CheckAuth(r *http.Request) error
}

type BasicAuth struct {
	user             string
	pass             string
	expectedUserHash [32]byte
	expectedPassHash [32]byte
}

func NewAuthenticator() Authenticator {
	user, userOk := os.LookupEnv("PROXY_USER")
	pass, passOk := os.LookupEnv("PROXY_PASS")

	if !userOk && !passOk {
		return nil
	}

	return &BasicAuth{
		user:             user,
		pass:             pass,
		expectedUserHash: sha256.Sum256([]byte(user)),
		expectedPassHash: sha256.Sum256([]byte(pass)),
	}
}

func (a *BasicAuth) extractCredentials(authHeader string) (string, string, error) {
	const prefix = "Basic "
	if !strings.HasPrefix(authHeader, prefix) {
		return "", "", ErrUnauthorized
	}

	rawPayload := authHeader[len(prefix):]
	if len(rawPayload) > 16384 {
		return "", "", ErrUnauthorized
	}

	payload, err := base64.StdEncoding.DecodeString(rawPayload)
	if err != nil {
		return "", "", ErrUnauthorized
	}

	userVal, passVal, ok := strings.Cut(string(payload), ":")
	if !ok {
		return "", "", ErrUnauthorized
	}
	return userVal, passVal, nil
}

func (a *BasicAuth) verifyCredentials(userVal, passVal string) bool {
	userHash := sha256.Sum256([]byte(userVal))
	userMatch := subtle.ConstantTimeCompare(userHash[:], a.expectedUserHash[:]) == 1

	passHash := sha256.Sum256([]byte(passVal))
	passMatch := subtle.ConstantTimeCompare(passHash[:], a.expectedPassHash[:]) == 1

	return userMatch && passMatch
}

func (a *BasicAuth) CheckAuth(r *http.Request) error {
	authHeader := r.Header.Get("Proxy-Authorization")
	if authHeader == "" {
		return ErrUnauthorized
	}

	userVal, passVal, err := a.extractCredentials(authHeader)
	if err != nil {
		return err
	}

	if !a.verifyCredentials(userVal, passVal) {
		return ErrUnauthorized
	}

	return nil
}
