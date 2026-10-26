package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/VariatezzZ/geoip/internal/config"
	"github.com/VariatezzZ/geoip/internal/fetch"
	"github.com/VariatezzZ/geoip/internal/ipinfo"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("fetch IPinfo failed: %v", err)
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

	result, err := ipinfo.Fetch(ctx, cfg.IPInfo, fetch.New())
	if err != nil {
		return err
	}
	if err := ipinfo.Save(cfg.IPInfo.Output, result.Data); err != nil {
		return err
	}

	sum := sha256.Sum256(result.Data)
	log.Printf("saved %s: %d bytes, sha256 %s, source %s", cfg.IPInfo.Output, len(result.Data), hex.EncodeToString(sum[:]), result.Mirror)
	return nil
}
