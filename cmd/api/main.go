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
	"be-dikasa/internal/refund"
	"be-dikasa/internal/registration"
	"be-dikasa/internal/reports"
	"be-dikasa/internal/reservation"
	"be-dikasa/internal/sales"
	"be-dikasa/internal/savedorder"
	"be-dikasa/internal/server"
	"be-dikasa/internal/shared/supabaseauth"
	"be-dikasa/internal/shared/supabasestorage"
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

	supabaseAuth, err := supabaseauth.NewClient(
		configuration.Supabase.URL,
		configuration.Supabase.PublishableKey,
		configuration.Supabase.SecretKey,
	)
	if err != nil {
		return err
	}

	userRepository := user.NewGORMRepository(postgres)
	supabaseImages, err := supabasestorage.NewUploader(
		configuration.Supabase.URL,
		configuration.Supabase.StorageBucket,
		configuration.Supabase.SecretKey,
	)
	if err != nil {
		return err
	}
	if err = supabaseImages.VerifyBucket(ctx); err != nil {
		return err
	}
	imageStore := catalog.NewSupabaseImageStore(supabaseImages)
	avatarStore := user.NewSupabaseAvatarStore(supabaseImages)
	proofStore := cash.NewSupabaseMovementProofStore(supabaseImages)
	userService := user.NewService(userRepository, user.WithAvatarStore(avatarStore))
	catalogRepository := catalog.NewGORMRepository(postgres)
	catalogService := catalog.NewService(catalogRepository, userRepository, imageStore)
	cashRepository := cash.NewGORMRepository(postgres)
	cashService := cash.NewService(cashRepository, userRepository, time.Now)
	paymentRepository := payment.NewGORMRepository(postgres)
	paymentService := payment.NewService(paymentRepository, userRepository)
	authService := auth.NewService(userRepository, supabaseAuth)
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
		StockHandler:        catalog.NewStockHandler(catalog.NewStockService(postgres, userRepository)),
		ExportHandler:       catalog.NewExportHandler(catalogService),
		AuthHandler:         auth.NewHandler(authService, sharedvalidator.New()),
		CashHandler:         cash.NewHandler(cashService, sharedvalidator.New()),
		ReportsHandler:      reports.NewHandler(reports.NewService(reports.NewGORMRepository(postgres), userRepository, time.Now)),
		MovementHandler:     cash.NewMovementHandler(cash.NewMovementService(cash.NewMovementRepository(postgres), userRepository, proofStore, time.Now)),
		CashClosingHandler:  cash.NewReconciliationHandler(cash.NewReconciliationService(cash.NewReconciliationRepository(postgres), userRepository)),
		CatalogHandler:      catalog.NewHandler(catalogService, sharedvalidator.New()),
		RefundHandler:       refund.NewHandler(refund.NewService(refund.NewGORMRepository(postgres), userRepository)),
		SavedOrderHandler:   savedorder.NewHandler(savedorder.NewService(savedorder.NewGORMRepository(postgres), userRepository)),
		PaymentHandler:      payment.NewHandler(paymentService),
		RegistrationHandler: registration.NewHandler(registrationService, sharedvalidator.New()),
		ReservationHandler:  reservation.NewHandler(reservationService, sharedvalidator.New()),
		SalesHandler:        sales.NewHandler(salesService),
		UserHandler:         user.NewHandler(userService),
		TokenVerifier:       server.NewSupabaseTokenVerifier(supabaseAuth, userRepository),
		CORSOrigins:         configuration.CORSOrigins,
	})

	address := ":" + configuration.Port
	httpServer := &http.Server{
		Addr:              address,
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
