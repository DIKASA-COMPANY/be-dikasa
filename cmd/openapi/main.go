// Command openapi mengekspor spesifikasi Huma ke api/openapi.yaml.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"

	"be-dikasa/internal/auth"
	"be-dikasa/internal/registration"
	"be-dikasa/internal/server"
	sharedvalidator "be-dikasa/internal/shared/validator"
	"be-dikasa/internal/user"
)

type schemaSignInProvider struct{}

func (schemaSignInProvider) SignIn(context.Context, string, string) (auth.Session, error) {
	return auth.Session{}, nil
}

type schemaRegistrationProvider struct{}

func (schemaRegistrationProvider) Register(context.Context, registration.Registration) (registration.Result, error) {
	return registration.Result{}, nil
}

func (schemaRegistrationProvider) ConfirmOTP(context.Context, string, string) error { return nil }

func (schemaRegistrationProvider) ResendOTP(context.Context, string) (registration.Result, error) {
	return registration.Result{}, nil
}

type schemaUserRepository struct{}

func (schemaUserRepository) FindByIdentifier(context.Context, string) (*user.User, error) {
	return nil, user.ErrNotFound
}

func (schemaUserRepository) FindByID(context.Context, string) (*user.User, error) {
	return nil, user.ErrNotFound
}

func main() {
	output := flag.String("output", "api/openapi.yaml", "lokasi file OpenAPI")
	flag.Parse()

	userService := user.NewService(schemaUserRepository{})
	router := server.NewRouter(server.Dependencies{
		AuthHandler:         auth.NewHandler(schemaSignInProvider{}, sharedvalidator.New()),
		RegistrationHandler: registration.NewHandler(schemaRegistrationProvider{}, sharedvalidator.New()),
		UserHandler:         user.NewHandler(userService),
	})
	request := httptest.NewRequest(http.MethodGet, "/dikasa/v1/openapi.yaml", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		panic(fmt.Errorf("menghasilkan OpenAPI: status %d: %s", response.Code, response.Body.String()))
	}
	if response.Body.Len() == 0 {
		panic(errors.New("menghasilkan OpenAPI: dokumen kosong"))
	}
	if err := os.WriteFile(*output, response.Body.Bytes(), 0o644); err != nil {
		panic(fmt.Errorf("menulis OpenAPI: %w", err))
	}
}
