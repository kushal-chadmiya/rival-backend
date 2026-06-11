package app

import (
	"context"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"taskboard-backend/db"
	"taskboard-backend/internal/activity"
	"taskboard-backend/internal/auth"
	"taskboard-backend/internal/config"
	"taskboard-backend/internal/httpapi"
	"taskboard-backend/internal/realtime"
	"taskboard-backend/internal/storage"
	"taskboard-backend/internal/store"
)

// App is the bootstrapped application container.
type App struct {
	Config   config.Config
	Router   http.Handler
	db       *pgxpool.Pool
	verifier auth.Verifier
}

// New initializes the application.
func New(ctx context.Context) (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}

	if err := db.Apply(ctx, pool); err != nil {
		return nil, fmt.Errorf("apply database schema: %w", err)
	}

	verifier, err := auth.NewVerifier(ctx, cfg.SupabaseJWTSecret, cfg.SupabaseJWKSURL)
	if err != nil {
		return nil, fmt.Errorf("create jwt verifier: %w", err)
	}

	taskStore := store.New(pool)
	activityStore := activity.New(pool)
	authClient := auth.NewClient(cfg.SupabaseURL, cfg.SupabaseAnonKey)
	hub := realtime.NewHub()

	var fileStorage *storage.Client
	if cfg.SupabaseServiceRoleKey != "" {
		fileStorage = storage.NewClient(cfg.SupabaseURL, cfg.SupabaseServiceRoleKey, cfg.StorageBucket)
	}

	return &App{
		Config: cfg,
		Router: httpapi.NewRouter(httpapi.RouterDependencies{
			FrontendURL:    cfg.FrontendURL,
			Verifier:       verifier,
			AuthClient:     authClient,
			TaskStore:      taskStore,
			ActivityStore:  activityStore,
			FileStorage:    fileStorage,
			Hub:            hub,
			MaxUploadBytes:   cfg.MaxUploadBytes,
			AllowViewAsAdmin: cfg.AllowViewAsAdmin,
		}),
		db:       pool,
		verifier: verifier,
	}, nil
}

// Close releases application resources.
func (a *App) Close() {
	if a.verifier != nil {
		_ = a.verifier.Close(context.Background())
	}
	if a.db != nil {
		a.db.Close()
	}
}
