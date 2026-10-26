package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/VariatezzZ/geoip/internal/build"
	"github.com/VariatezzZ/geoip/internal/config"
	"github.com/VariatezzZ/geoip/internal/fetch"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("build failed: %v", err)
	}
}

func run() error {
	configPath := flag.String("config", "config.yaml", "path to the build configuration")
	flag.Parse()
	if flag.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %v", flag.Args())
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	report, err := build.Run(ctx, cfg, fetch.New())
	if err != nil {
		return err
	}

	for _, category := range []string{"CN", "PRIVATE"} {
		log.Printf("category %s: %d prefixes", category, report.Categories[category])
	}
	for _, output := range report.Outputs {
		log.Printf("output %s: %d bytes, sha256 %s", output.Path, output.Size, output.SHA256)
	}
	return nil
}
