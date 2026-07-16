package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/arkeo/arkeo/internal/cache"
	"github.com/arkeo/arkeo/internal/web"
)

var webCmd = &cobra.Command{
	Use:   "web",
	Short: "Launch the Arkeo web UI",
	Long: `Launch a local web application that provides a full interactive interface
for browsing timelines, managing connectors, and configuring browser domain exclusions.

The web UI uses a dark theme and requires no JavaScript frameworks — just vanilla JS
for API calls and rendering.`,
	Args: cobra.NoArgs,
	Run:  runWebCommand,
}

func init() {
	// --addr flag is registered on rootCmd as a persistent flag
}

func runWebCommand(cmd *cobra.Command, args []string) {
	configManager, registry := initializeSystem()

	// Initialize cache
	var activityCache *cache.Cache
	configDir, err := configManager.GetConfigDir()
	if err == nil {
		cachePath := filepath.Join(configDir, "cache.db")
		activityCache, err = cache.New(cachePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not open cache: %v\n", err)
		}
		defer activityCache.Close()
	}

	// Create and start the web server
	server := web.New(configManager, registry, activityCache)

	// Handle graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		fmt.Fprintln(os.Stderr, "\nShutting down...")
		server.Shutdown(context.Background())
	}()

	if err := server.ListenAndServe(webAddr); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}