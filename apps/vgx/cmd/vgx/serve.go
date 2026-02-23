package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/spf13/cobra"
	"github.com/vibeguard/vgx/internal/api"
	"github.com/vibeguard/vgx/internal/config"
	"github.com/vibeguard/vgx/internal/db"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the VGX API server with embedded dashboard",
	RunE:  runServe,
}

func init() {
	serveCmd.Flags().IntP("port", "p", 8080, "Port to listen on")
}

func runServe(cmd *cobra.Command, _ []string) error {
	port, _ := cmd.Flags().GetInt("port")
	cfg := config.Load()

	ctx := context.Background()
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("database connection failed: %w", err)
	}
	defer pool.Close()

	router := api.NewRouter(cfg, pool)

	addr := fmt.Sprintf(":%d", port)
	log.Printf("VGX server starting on %s", addr)
	return http.ListenAndServe(addr, router)
}
