package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"chatgpt2api/internal/config"
	"chatgpt2api/internal/httpapi"
	"chatgpt2api/internal/service"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "migrate-images-r2" {
		runR2ImageMigration()
		return
	}

	app, err := httpapi.NewApp()
	if err != nil {
		log.Fatalf("init app: %v", err)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}
	logger := app.Logger()

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           app.Handler(),
		ReadHeaderTimeout: 30 * time.Second,
	}

	go func() {
		logger.Info("starting server", "addr", ":"+port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("listen failed", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("server shutdown failed", "error", err)
	}
	app.Close()
}

func runR2ImageMigration() {
	cfg, err := config.NewStore()
	if err != nil {
		log.Fatalf("init config: %v", err)
	}
	backend, err := cfg.StorageBackend()
	if err != nil {
		log.Fatalf("init storage: %v", err)
	}
	var paths []string
	root := cfg.ImagesDir()
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err == nil {
			paths = append(paths, filepath.ToSlash(rel))
		}
		return nil
	})
	result, err := service.NewImageService(cfg, backend).UploadImagesToObjectStorage(context.Background(), paths, service.ImageAccessScope{All: true})
	if err != nil {
		log.Fatalf("migrate images to R2: %v", err)
	}
	fmt.Printf("R2 image migration finished: uploaded=%v missing=%v failed=%v\n", result["uploaded"], result["missing"], result["failed"])
	for _, item := range result["errors"].([]map[string]any) {
		fmt.Printf("%v: %v\n", item["path"], item["error"])
	}
	if failed, ok := result["failed"].(int); ok && failed > 0 {
		os.Exit(1)
	}
}
