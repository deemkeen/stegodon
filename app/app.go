package app

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"charm.land/wish/v2"
	"charm.land/wish/v2/bubbletea"
	"charm.land/wish/v2/logging"
	"github.com/charmbracelet/ssh"
	"github.com/deemkeen/stegodon/activitypub"
	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/middleware"
	"github.com/deemkeen/stegodon/util"
	"github.com/deemkeen/stegodon/web"
)

// headToGetWrapper wraps an http.Handler to convert HEAD requests to GET.
// This is needed because Gin's router matches routes BEFORE middleware runs,
// so a middleware-based approach doesn't work for HEAD support.
// Go's http package automatically strips the body for HEAD responses.
func headToGetWrapper(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "HEAD" {
			r.Method = "GET"
		}
		h.ServeHTTP(w, r)
	})
}

// App represents the main application with all its servers and dependencies
type App struct {
	config             *util.AppConfig
	sshServer          *ssh.Server
	httpServer         *http.Server
	done               chan os.Signal
	stopDeliveryWorker func() // Stop function for ActivityPub delivery worker
}

// New creates a new App instance with the given configuration
func New(conf *util.AppConfig) (*App, error) {
	return &App{
		config: conf,
		done:   make(chan os.Signal, 1),
	}, nil
}

// Initialize sets up the database, runs migrations, and initializes servers
func (a *App) Initialize() error {
	// Run database migrations
	log.Println("Running database migrations...")
	database := db.GetDB()
	if err := database.RunActivityPubMigrations(); err != nil {
		log.Printf("Warning: Migration errors (may be normal if tables exist): %v", err)
	}
	log.Println("Database migrations complete")

	// Run key format migration (PKCS#1 to PKCS#8)
	log.Println("Checking for key format migration...")
	if err := database.MigrateKeysToPKCS8(); err != nil {
		log.Printf("Warning: Key migration encountered errors: %v", err)
		log.Println("You may need to manually review the migration. See logs above for details.")
	} else {
		log.Println("Key format migration complete")
	}

	// Run duplicate follows cleanup migration
	log.Println("Checking for duplicate follows...")
	if err := database.MigrateDuplicateFollows(); err != nil {
		log.Printf("Warning: Duplicate follows migration encountered errors: %v", err)
		log.Println("You may need to manually review the migration. See logs above for details.")
	} else {
		log.Println("Duplicate follows migration complete")
	}

	// Run local reply counts migration
	log.Println("Checking for uncounted local replies...")
	if err := database.MigrateLocalReplyCounts(); err != nil {
		log.Printf("Warning: Local reply counts migration encountered errors: %v", err)
	} else {
		log.Println("Local reply counts migration complete")
	}

	// Run performance indexes migration
	if err := database.MigratePerformanceIndexes(); err != nil {
		log.Printf("Warning: Performance indexes migration encountered errors: %v", err)
	}

	// Run FTS5 search index migration
	if err := database.MigrateFTS5Search(); err != nil {
		log.Printf("Warning: FTS5 search migration encountered errors: %v", err)
	}

	// Run orphan activities cleanup migration (fixes bug where Create activities
	// were stored before checking follow relationships)
	log.Println("Checking for orphan activities...")
	if err := database.MigrateOrphanActivities(); err != nil {
		log.Printf("Warning: Orphan activities migration encountered errors: %v", err)
	} else {
		log.Println("Orphan activities migration complete")
	}

	// Cleanup expired IP bans (IP addresses older than 60 days are cleared)
	database.CleanupExpiredIPBans()

	// Initialize SSH server
	sshKeyPath := util.ResolveFilePathWithSubdir(".ssh", "stegodonhostkey")
	log.Printf("Using SSH host key at: %s", sshKeyPath)

	sshServer, err := wish.NewServer(
		wish.WithAddress(fmt.Sprintf("%s:%d", a.config.Conf.Host, a.config.Conf.SshPort)),
		wish.WithHostKeyPath(sshKeyPath),
		wish.WithPublicKeyAuth(func(ssh.Context, ssh.PublicKey) bool { return true }),
		wish.WithMiddleware(
			bubbletea.Middleware(middleware.TeaHandler),
			middleware.MainTui(),
			middleware.AuthMiddleware(a.config),
			logging.MiddlewareWithLogger(log.Default()),
		),
	)
	if err != nil {
		return fmt.Errorf("failed to create SSH server: %w", err)
	}
	a.sshServer = sshServer

	// Initialize HTTP router and server
	router, err := web.Router(a.config)
	if err != nil {
		return fmt.Errorf("failed to initialize HTTP router: %w", err)
	}

	a.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", a.config.Conf.HttpPort),
		Handler: headToGetWrapper(router),
	}

	return nil
}

// Start starts all servers and blocks until a shutdown signal is received
func (a *App) Start() error {
	// Start ActivityPub delivery worker if enabled
	if a.config.Conf.WithAp {
		a.stopDeliveryWorker = activitypub.StartDeliveryWorker(a.config)
	}

	// Setup signal handling
	signal.Notify(a.done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	// Start SSH server
	log.Printf("Starting SSH server on %s:%d", a.config.Conf.Host, a.config.Conf.SshPort)
	go func() {
		if err := a.sshServer.ListenAndServe(); err != nil && err != ssh.ErrServerClosed {
			log.Fatalf("SSH server error: %v", err)
		}
	}()

	// Start HTTP server
	log.Printf("Starting HTTP server on %s:%d", a.config.Conf.Host, a.config.Conf.HttpPort)
	go func() {
		if err := a.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-a.done
	log.Println("Shutdown signal received")

	return a.Shutdown()
}

// Shutdown gracefully stops all servers with a 30 second timeout
func (a *App) Shutdown() error {
	log.Println("Initiating graceful shutdown...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var shutdownErr error

	// Stop ActivityPub delivery worker first
	if a.stopDeliveryWorker != nil {
		log.Println("Stopping ActivityPub delivery worker...")
		a.stopDeliveryWorker()
	}

	// Shutdown HTTP server (stop accepting new requests)
	log.Println("Stopping HTTP server...")
	if err := a.httpServer.Shutdown(ctx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
		shutdownErr = err
	} else {
		log.Println("HTTP server stopped gracefully")
	}

	// Shutdown SSH server
	log.Println("Stopping SSH server...")
	if err := a.sshServer.Shutdown(ctx); err != nil {
		log.Printf("SSH server shutdown error: %v", err)
		if shutdownErr == nil {
			shutdownErr = err
		}
	} else {
		log.Println("SSH server stopped gracefully")
	}

	log.Println("All servers stopped")
	return shutdownErr
}
