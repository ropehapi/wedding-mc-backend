// @title          Wedding MC API
// @version        1.0
// @description    API para gestão de casamentos — convidados, presentes e página pública.
// @host           localhost:8080
// @BasePath       /
//
// @securityDefinitions.apikey BearerAuth
// @in             header
// @name           Authorization

package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	httpSwagger "github.com/swaggo/http-swagger"

	"github.com/ropehapi/wedding-mc/internal/config"
	_ "github.com/ropehapi/wedding-mc/docs"
	"github.com/ropehapi/wedding-mc/internal/handler"
	"github.com/ropehapi/wedding-mc/internal/middleware"
	"github.com/ropehapi/wedding-mc/internal/repository"
	"github.com/ropehapi/wedding-mc/internal/service"
)

func main() {
	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).With().Timestamp().Logger()
	log.Logger = logger

	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load config")
	}

	if err := config.RunMigrations(cfg); err != nil {
		log.Fatal().Err(err).Msg("failed to run migrations")
	}

	db, err := config.NewDB(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to database")
	}
	defer db.Close()

	// --- Repositories ---
	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewRefreshTokenRepository(db)
	weddingRepo := repository.NewWeddingRepository(db)
	guestRepo := repository.NewGuestRepository(db)
	giftRepo := repository.NewGiftRepository(db)
	tableRepo := repository.NewTableRepository(db)

	// --- Services ---
	baseURL := fmt.Sprintf("http://localhost:%s/uploads", cfg.Port)
	storageSvc := service.NewLocalStorage(cfg.LocalStoragePath, baseURL)

	weddingSvc := service.NewWeddingService(weddingRepo, storageSvc)
	authSvc := service.NewAuthService(userRepo, tokenRepo, weddingSvc, cfg.JWTSecret, cfg.JWTExpiry, cfg.RefreshExpiry)
	guestSvc := service.NewGuestService(guestRepo, weddingRepo)
	giftSvc := service.NewGiftService(giftRepo, weddingRepo)
	tableSvc := service.NewTableService(tableRepo, guestRepo, weddingRepo)

	// --- Handlers ---
	authHandler := handler.NewAuthHandler(authSvc)
	weddingHandler := handler.NewWeddingHandler(weddingSvc)
	guestHandler := handler.NewGuestHandler(guestSvc)
	giftHandler := handler.NewGiftHandler(giftSvc)
	publicHandler := handler.NewPublicHandler(weddingSvc, guestSvc, giftSvc)
	tableHandler := handler.NewTableHandler(tableSvc)

	// --- Router ---
	r := chi.NewRouter()

	r.Use(chiMiddleware.RealIP)
	r.Use(middleware.Logger(logger))
	r.Use(middleware.Recoverer(logger))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{cfg.AllowedOrigins},
		AllowedMethods:   []string{"GET", "POST", "PATCH", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
	}))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		handler.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Get("/swagger/*", httpSwagger.Handler())

	r.Route("/v1", func(r chi.Router) {
		// Auth — public
		r.Route("/auth", func(r chi.Router) {
			r.Post("/register", authHandler.Register)
			r.Post("/login", authHandler.Login)
			r.Post("/refresh", authHandler.Refresh)
			r.With(middleware.Auth(cfg.JWTSecret)).Post("/logout", authHandler.Logout)
		})

		// Wedding — protected
		r.Route("/wedding", func(r chi.Router) {
			r.Use(middleware.Auth(cfg.JWTSecret))
			r.Get("/", weddingHandler.Get)
			r.Post("/", weddingHandler.Create)
			r.Patch("/", weddingHandler.Update)
			r.Post("/photos", weddingHandler.UploadPhoto)
			r.Delete("/photos/{photoID}", weddingHandler.DeletePhoto)
			r.Patch("/photos/{photoID}/cover", weddingHandler.SetCoverPhoto)
		})

		// Guests — protected
		r.Route("/guests", func(r chi.Router) {
			r.Use(middleware.Auth(cfg.JWTSecret))
			r.Post("/", guestHandler.Create)
			r.Get("/", guestHandler.List)
			r.Get("/summary", guestHandler.Summary)
			r.Patch("/{guestID}", guestHandler.Update)
			r.Delete("/{guestID}", guestHandler.Delete)
		})

		// Gifts — protected
		r.Route("/gifts", func(r chi.Router) {
			r.Use(middleware.Auth(cfg.JWTSecret))
			r.Post("/", giftHandler.Create)
			r.Get("/", giftHandler.List)
			r.Get("/summary", giftHandler.Summary)
			r.Patch("/{giftID}", giftHandler.Update)
			r.Delete("/{giftID}", giftHandler.Delete)
			r.Delete("/{giftID}/reserve", giftHandler.CancelReserve)
		})

		// Tables — protected
		r.Route("/tables", func(r chi.Router) {
			r.Use(middleware.Auth(cfg.JWTSecret))
			r.Post("/", tableHandler.Create)
			r.Get("/", tableHandler.List)
			r.Patch("/{tableID}", tableHandler.Update)
			r.Delete("/{tableID}", tableHandler.Delete)
			r.Put("/{tableID}/guests/{guestID}", tableHandler.AssignGuest)
			r.Delete("/{tableID}/guests/{guestID}", tableHandler.UnassignGuest)
		})

		// Public — no auth
		r.Route("/public", func(r chi.Router) {
			r.Get("/{slug}", publicHandler.GetWedding)
			r.Get("/{slug}/guests", publicHandler.ListGuests)
			r.Post("/{slug}/guests/validate-code", publicHandler.ValidateCode)
			r.Post("/{slug}/guests/{guestID}/rsvp", publicHandler.RSVP)
			r.Get("/{slug}/gifts", publicHandler.ListGifts)
			r.Post("/{slug}/gifts/{giftID}/reserve", publicHandler.ReserveGift)
		})
	})

	// Serve uploaded files (local storage only)
	if cfg.StorageDriver == "local" {
		r.Handle("/uploads/*", http.StripPrefix("/uploads/",
			http.FileServer(http.Dir(cfg.LocalStoragePath))))
	}

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		log.Info().Str("addr", srv.Addr).Msg("server starting")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("server error")
		}
	}()

	<-quit
	log.Info().Msg("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("server forced to shutdown")
	}
	log.Info().Msg("server stopped")
}
