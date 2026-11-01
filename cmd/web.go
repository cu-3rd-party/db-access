package cmd

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"git.cu3rd.ru/cu3rd/db-access/models/accesslog"
	"git.cu3rd.ru/cu3rd/db-access/models/db"
	"git.cu3rd.ru/cu3rd/db-access/modules/setting"
	"git.cu3rd.ru/cu3rd/db-access/routers"
	"git.cu3rd.ru/cu3rd/db-access/routers/httpmw"
	access_service "git.cu3rd.ru/cu3rd/db-access/services/access"
	"git.cu3rd.ru/cu3rd/db-access/services/provision"

	"github.com/urfave/cli/v3"
)

const (
	expireInterval    = time.Minute
	privilegeInterval = 10 * time.Minute
	logPurgeInterval  = 24 * time.Hour
	shutdownTimeout   = 10 * time.Second
)

func every(ctx context.Context, interval time.Duration, name string, fn func(context.Context) error) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := fn(ctx); err != nil {
				slog.Error(name, "err", err)
			}
		}
	}
}

func cmdWeb() *cli.Command {
	return &cli.Command{
		Name:  "web",
		Usage: "Start the web server",
		Action: func(parent context.Context, _ *cli.Command) error {
			slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
			setting.Load()
			if err := setting.Validate(); err != nil {
				return err
			}
			ctx, stop := signal.NotifyContext(parent, syscall.SIGINT, syscall.SIGTERM)
			defer stop()
			if err := db.InitEngine(ctx); err != nil {
				return err
			}
			provisioner, err := provision.NewPostgres()
			if err != nil {
				return err
			}
			defer provisioner.Close()
			access_service.SetProvisioner(provisioner)
			if err := access_service.Bootstrap(ctx); err != nil {
				return err
			}

			logsWritten := httpmw.StartAccessLogWriter(ctx)
			go every(ctx, expireInterval, "expire write access", func(ctx context.Context) error {
				return access_service.ExpireWrites(ctx, time.Now())
			})
			go every(ctx, privilegeInterval, "apply privileges", access_service.Bootstrap)
			go every(ctx, logPurgeInterval, "purge access log", func(ctx context.Context) error {
				_, err := accesslog.DeleteOlderThan(ctx, time.Now().Add(-setting.AccessLogRetention))
				return err
			})
			// the pod may have been down when grants were due
			if err := access_service.ExpireWrites(ctx, time.Now()); err != nil {
				slog.Error("expire write access", "err", err)
			}

			slog.Info("listening", "addr", setting.HTTPAddr)
			srv := &http.Server{
				Addr:              setting.HTTPAddr,
				Handler:           routers.NormalRoutes(),
				ReadHeaderTimeout: 10 * time.Second,
			}
			go func() {
				<-ctx.Done()
				shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
				defer cancel()
				_ = srv.Shutdown(shutdownCtx)
			}()
			if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
				return err
			}
			<-logsWritten
			return nil
		},
	}
}

func cmdMigrate() *cli.Command {
	return &cli.Command{
		Name:  "migrate",
		Usage: "Apply database migrations and exit",
		Action: func(ctx context.Context, _ *cli.Command) error {
			setting.Load()
			return db.InitEngine(ctx)
		},
	}
}
