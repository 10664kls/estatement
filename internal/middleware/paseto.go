package middleware

import (
	"errors"
	"time"

	"aidanwoods.dev/go-paseto"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type pasetoExtractor func(echo.Context) (string, error)

// pasetoFromHeader returns a `pasetoExtractor` that extracts token from the request header.
func pasetoFromHeader(header string, authScheme string) pasetoExtractor {
	return func(c echo.Context) (string, error) {
		auth := c.Request().Header.Get(header)
		l := len(authScheme)
		if len(auth) > l+1 && auth[:l] == authScheme {
			return auth[l+1:], nil
		}
		return "", errors.New("missing or malformed paseto")
	}
}

// PASETOConfig defines the config for PASETO middleware.
type PASETOConfig struct {
	// Skipper defines a function to skip middleware.
	Skipper middleware.Skipper

	// ErrorHandler defines a function which is executed for an invalid token.
	// It may be used to define a custom PASETO error.
	ErrorHandler func(echo.Context, error) error

	// SymmetricKey is the key used to sign and decrypted PASETO token.
	SymmetricKey paseto.V4SymmetricKey

	// Implicit are bytes used to calculate the encrypted token, but which are not
	// present in the final token (or its decrypted value).
	Implicit []byte

	// Rules are the rules used to validate the token.
	Rules []paseto.Rule

	// ContextKey key to store token information *paseto.Token into echo context.
	// Optional. Default value "token".
	ContextKey string
}

// PASETO returns a PASETO auth middleware.
func PASETO(cfg PASETOConfig) echo.MiddlewareFunc {
	if cfg.Skipper == nil {
		cfg.Skipper = middleware.DefaultSkipper
	}
	if cfg.ContextKey == "" {
		cfg.ContextKey = "token"
	}

	extractor := pasetoFromHeader(echo.HeaderAuthorization, "Bearer")

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if cfg.Skipper(c) {
				return next(c)
			}

			tainted, err := extractor(c)
			if err != nil {
				if cfg.ErrorHandler != nil {
					return cfg.ErrorHandler(c, err)
				}

				return status.Error(
					codes.Unauthenticated,
					"Your provided token not valid, Please provide a valid token.",
				)
			}

			rules := append(cfg.Rules, paseto.NotExpired(), paseto.ValidAt(time.Now()))
			parser := paseto.MakeParser(rules)
			token, err := parser.ParseV4Local(cfg.SymmetricKey, tainted, cfg.Implicit)
			if err != nil {
				if cfg.ErrorHandler != nil {
					return cfg.ErrorHandler(c, err)
				}

				return status.Error(
					codes.Unauthenticated,
					"Your provided token not valid, Please provide a valid token.",
				)
			}

			c.Set(cfg.ContextKey, token)
			return next(c)
		}
	}
}
