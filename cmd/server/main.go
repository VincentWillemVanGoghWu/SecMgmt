package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"secmgmt_go/internal/bootstrap"

	"go.uber.org/zap"
)

func main() {
	rootDir, err := resolveRootDir()
	if err != nil {
		log.Fatalf("resolve root dir: %v", err)
	}

	app, err := bootstrap.Build(rootDir)
	if err != nil {
		log.Fatalf("build app: %v", err)
	}
	closed := false
	closeApp := func() {
		if closed {
			return
		}
		closed = true
		if app.Close != nil {
			app.Close()
		}
	}
	defer func() {
		closeApp()
		_ = app.Logger.Sync()
	}()

	app.Logger.Info("secmgmt-go server starting",
		zap.String("addr", app.Config.HTTPAddr()),
	)

	server := &http.Server{
		Addr:    app.Config.HTTPAddr(),
		Handler: app.Router,
	}
	serverErrors := make(chan error, 1)
	go func() {
		err := server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
			return
		}
		serverErrors <- nil
	}()

	shutdownSignals := make(chan os.Signal, 1)
	signal.Notify(shutdownSignals, os.Interrupt, syscall.SIGTERM)
	serverStopped := false
	select {
	case err := <-serverErrors:
		serverStopped = true
		if err != nil {
			app.Logger.Fatal("run http server", zap.Error(err))
		}
	case sig := <-shutdownSignals:
		app.Logger.Info("secmgmt-go server shutting down", zap.String("signal", sig.String()))
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			app.Logger.Warn("shutdown http server", zap.Error(err))
		}
	}

	closeApp()
	if !serverStopped {
		if err := <-serverErrors; err != nil {
			app.Logger.Fatal("run http server", zap.Error(err))
		}
	}
}

func resolveRootDir() (string, error) {
	if cwd, err := os.Getwd(); err == nil {
		return cwd, nil
	}

	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Dir(filepath.Dir(filepath.Dir(executable))), nil
}
