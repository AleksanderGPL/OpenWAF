package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"OpenWAF/internal/api"
	"OpenWAF/internal/auth"
	"OpenWAF/internal/database"
	"OpenWAF/internal/proxy"
	"OpenWAF/internal/services"

	"github.com/gofiber/fiber/v3"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	path := os.Getenv("DATABASE_PATH")
	if path == "" {
		path = "data/openwaf.db"
	}
	db, err := database.Open(path)
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	secure := defaultSecureCookies
	if value, ok := os.LookupEnv("AUTH_COOKIE_SECURE"); ok {
		secure, err = strconv.ParseBool(value)
		if err != nil {
			return errors.New("AUTH_COOKIE_SECURE must be a boolean")
		}
	}
	store := database.NewStore(db)
	authService, err := auth.New(store)
	if err != nil {
		return err
	}
	app := fiber.New(fiber.Config{ErrorHandler: api.ErrorHandler, BodyLimit: 16 * 1024})

	proxyService := proxy.New(store)
	defer proxyService.Close()
	if err := registerAPI(app, auth.NewHandler(authService, secure), services.NewHandler(services.New(store))); err != nil {
		return err
	}

	serveFrontend(app)
	proxyAddress := os.Getenv("PROXY_LISTEN_ADDRESS")
	if proxyAddress == "" {
		proxyAddress = ":8080"
	}
	proxyListener, err := net.Listen("tcp", proxyAddress)
	if err != nil {
		return err
	}
	defer proxyListener.Close()
	adminListener, err := net.Listen("tcp", listenAddress)
	if err != nil {
		return err
	}
	defer adminListener.Close()
	proxyServer := &http.Server{
		Handler: proxyService, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 90 * time.Second,
	}
	defer proxyServer.Close()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	serverErrors := make(chan error, 2)
	go func() { serverErrors <- proxyServer.Serve(proxyListener) }()
	go func() { serverErrors <- app.Listener(adminListener) }()
	log.Printf("proxy listening on %s", proxyListener.Addr())
	select {
	case err = <-serverErrors:
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	shutdownErrors := make(chan error, 1)
	go func() { shutdownErrors <- app.ShutdownWithContext(shutdownCtx) }()
	proxyShutdownErr := proxyServer.Shutdown(shutdownCtx)
	adminShutdownErr := <-shutdownErrors
	if errors.Is(err, http.ErrServerClosed) {
		err = nil
	}
	return errors.Join(err, proxyShutdownErr, adminShutdownErr)
}
