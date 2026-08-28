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
	"be-dikasa/internal/cash"
	"be-dikasa/internal/catalog"
	"be-dikasa/internal/config"
	"be-dikasa/internal/database"
	"be-dikasa/internal/payment"
	"be-dikasa/internal/registration"
	"be-dikasa/internal/reservation"
	"be-dikasa/internal/sales"
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
	if err := config.LoadDotEnv(".env"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("memuat .env: %w", err)
	}
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
	var imageStore catalog.ImageStore = catalog.NewDisabledImageStore()
	var avatarStore user.AvatarStore = user.NewDisabledAvatarStore()
	if configuration.Firebase.StorageMode == config.ImageStorageFirebase {
		firebaseStore, closeStorage, storageErr := catalog.NewCloudFirebaseImageStore(ctx, configuration.Firebase.StorageBucket)
		if storageErr != nil {
			return storageErr
		}
		defer func() {
			if closeErr := closeStorage(); closeErr != nil {
				log.Printf("menutup Firebase Storage: %v", closeErr)
			}
		}()
		imageStore = firebaseStore
		firebaseAvatarStore, closeAvatarStorage, avatarStorageErr := user.NewCloudFirebaseAvatarStore(ctx, configuration.Firebase.StorageBucket)
		if avatarStorageErr != nil {
			return avatarStorageErr
		}
		defer func() {
			if closeErr := closeAvatarStorage(); closeErr != nil {
				log.Printf("menutup Firebase Avatar Storage: %v", closeErr)
			}
		}()
		avatarStore = firebaseAvatarStore
	}
	userService := user.NewService(userRepository, user.WithAvatarStore(avatarStore))
	catalogRepository := catalog.NewGORMRepository(postgres)
	catalogService := catalog.NewService(catalogRepository, userRepository, imageStore)
	cashRepository := cash.NewGORMRepository(postgres)
	cashService := cash.NewService(cashRepository, userRepository, time.Now)
	paymentRepository := payment.NewGORMRepository(postgres)
	paymentService := payment.NewService(paymentRepository, userRepository)
	tokenIssuer := auth.NewFirebaseTokenIssuer(firebaseAuth, configuration.Firebase.WebAPIKey)
	authService := auth.NewService(userRepository, tokenIssuer)
	registrationRepository := registration.NewGORMRepository(postgres)
	registrationMailer := registration.NewSMTPMailer(registration.SMTPConfig{
		Host: configuration.Mail.Host, Port: configuration.Mail.Port, Username: configuration.Mail.Username,
		Password: configuration.Mail.Password, From: configuration.Mail.From,
	})
	registrationService := registration.NewService(registrationRepository, registrationMailer, registration.NewHMACHasher(configuration.OTPSecret))
	reservationRepository := reservation.NewGORMRepository(postgres)
	reservationService := reservation.NewService(reservationRepository, userRepository, time.Now)
	salesRepository := sales.NewGORMRepository(postgres)
	salesService := sales.NewService(salesRepository, userRepository)
	router := server.NewRouter(server.Dependencies{
		AuthHandler:         auth.NewHandler(authService, sharedvalidator.New()),
		CashHandler:         cash.NewHandler(cashService, sharedvalidator.New()),
		CatalogHandler:      catalog.NewHandler(catalogService, sharedvalidator.New()),
		PaymentHandler:      payment.NewHandler(paymentService),
		RegistrationHandler: registration.NewHandler(registrationService, sharedvalidator.New()),
		ReservationHandler:  reservation.NewHandler(reservationService, sharedvalidator.New()),
		SalesHandler:        sales.NewHandler(salesService),
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
