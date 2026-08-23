package account

import "errors"

var (
	ErrInvalidRegistration = errors.New("registration is invalid")
	ErrInvalidCredentials  = errors.New("credentials are invalid")
	ErrEmailTaken          = errors.New("email is already registered")
	ErrUnauthenticated     = errors.New("authentication is required")
)
