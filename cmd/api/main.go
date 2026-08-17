// Command api menjalankan backend HTTP Dikasa.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"be-dikasa/internal/auth"
	"be-dikasa/internal/config"
	"be-dikasa/internal/database"
	"be-dikasa/internal/registration"
	"be-dikasa/internal/server"
	sharedvalidator "be-dikasa/internal/shared/validator"
	"be-dikasa/internal/user"
)

func main() {
	if err := run(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
	configuration, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}

	postgres, err := database.OpenPostgres(configuration.DatabaseURL)
	if err != nil {
		return fmt.Errorf("inisialisasi PostgreSQL: %w", err)
	}
	defer func() {
		if closeErr := database.ClosePostgres(postgres); closeErr != nil {
			log.Printf("menutup PostgreSQL: %v", closeErr)
		}
	}()

	firebaseAuth, err := database.OpenFirebase(ctx, configuration.Firebase.ProjectID)
	if err != nil {
		return err
	}

	userRepository := user.NewGORMRepository(postgres)
	userService := user.NewService(userRepository)
	tokenIssuer := auth.NewFirebaseTokenIssuer(firebaseAuth, configuration.Firebase.WebAPIKey)
	authService := auth.NewService(userRepository, tokenIssuer)
	registrationRepository := registration.NewGORMRepository(postgres)
	registrationMailer := registration.NewSMTPMailer(registration.SMTPConfig{
		Host: configuration.Mail.Host, Port: configuration.Mail.Port, Username: configuration.Mail.Username,
		Password: configuration.Mail.Password, From: configuration.Mail.From,
	})
	registrationService := registration.NewService(registrationRepository, registrationMailer, registration.NewHMACHasher(configuration.OTPSecret))
	router := server.NewRouter(server.Dependencies{
		AuthHandler:         auth.NewHandler(authService, sharedvalidator.New()),
		RegistrationHandler: registration.NewHandler(registrationService, sharedvalidator.New()),
		UserHandler:         user.NewHandler(userService),
		TokenVerifier:       firebaseAuth,
	})

	httpServer := &http.Server{
		Addr:              ":" + configuration.Port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Printf("server berjalan di http://localhost:%s", configuration.Port)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("menjalankan server: %w", err)
	}
	return nil
}
