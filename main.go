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

	"github.com/gofiber/fiber/v3"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

type StatsResponse struct {
	Status string `json:"status" required:"true"`
	Blocks int    `json:"blocks" required:"true"`
}

func registerAPI(app *fiber.App, authService *auth.Service, proxyService *proxy.Service) error {
	routes := api.New(app).Group("/api")
	authService.Register(routes)
	proxyService.Register(routes, authService.RequireAuth)
	routes.Handle(http.MethodGet, "/stats", api.Operation{
		ID: "getStats", Summary: "Get placeholder WAF statistics", Response: StatsResponse{}, Session: true, Errors: []int{401},
	}, authService.RequireAuth, func(c fiber.Ctx) error {
		return c.JSON(StatsResponse{Status: "WAF Active", Blocks: 127})
	})
	if err := registerDocs(routes); err != nil {
		return err
	}
	routes.Use(func(c fiber.Ctx) error { return fiber.ErrNotFound })
	return nil
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
	authService, err := auth.New(db, secure)
	if err != nil {
		return err
	}
	app := fiber.New(fiber.Config{ErrorHandler: auth.ErrorHandler, BodyLimit: 16 * 1024})

	proxyService := proxy.New(db)
	defer proxyService.Close()
	if err := registerAPI(app, authService, proxyService); err != nil {
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
