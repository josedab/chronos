// Chronos - Distributed Cron System
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/chronos/chronos/internal/api"
	"github.com/chronos/chronos/internal/config"
	"github.com/chronos/chronos/internal/dispatcher"
	"github.com/chronos/chronos/internal/raft"
	"github.com/chronos/chronos/internal/scheduler"
	"github.com/chronos/chronos/internal/storage"
	"github.com/rs/zerolog"
)

var (
	Version   = "dev"
	BuildTime = "unknown"
)

func main() {
	// Parse flags
	configPath := flag.String("config", "chronos.yaml", "Path to configuration file")
	showVersion := flag.Bool("version", false, "Show version information")
	flag.Parse()

	if *showVersion {
		fmt.Printf("Chronos %s (built %s)\n", Version, BuildTime)
		os.Exit(0)
	}

	// Setup logger
	logger := setupLogger()

	logger.Info().
		Str("version", Version).
		Str("build_time", BuildTime).
		Msg("Starting Chronos")

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Fatal().Err(err).Str("config", *configPath).Msg("Failed to load configuration")
	}

	// Create data directory
	if err := os.MkdirAll(cfg.Cluster.DataDir, 0755); err != nil {
		logger.Fatal().Err(err).Str("data_dir", cfg.Cluster.DataDir).Msg("Failed to create data directory")
	}

	// Initialize storage
	store, err := storage.NewStore(cfg.Cluster.DataDir)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize storage")
	}
	defer store.Close()

	// Initialize dispatcher
	dispatcherCfg := &dispatcher.Config{
		NodeID:          cfg.Cluster.NodeID,
		Timeout:         cfg.Dispatcher.HTTP.Timeout.Duration(),
		MaxIdleConns:    cfg.Dispatcher.HTTP.MaxIdleConns,
		IdleConnTimeout: cfg.Dispatcher.HTTP.IdleConnTimeout.Duration(),
		MaxConcurrent:   cfg.Dispatcher.Concurrency.MaxConcurrentJobs,
	}
	disp := dispatcher.New(dispatcherCfg)

	// Initialize scheduler
	schedulerCfg := &scheduler.Config{
		TickInterval:    cfg.Scheduler.TickInterval.Duration(),
		MissedRunPolicy: scheduler.MissedRunPolicy(cfg.Scheduler.MissedRunPolicy),
	}
	sched := scheduler.New(store, disp, logger, schedulerCfg)

	// Initialize Raft if peers are configured
	var raftNode *raft.Node
	if len(cfg.Cluster.Raft.Peers) > 0 {
		raftCfg := &raft.Config{
			NodeID:            cfg.Cluster.NodeID,
			RaftDir:           cfg.Cluster.DataDir + "/raft",
			RaftAddress:       cfg.Cluster.Raft.Address,
			Peers:             cfg.Cluster.Raft.Peers,
			HeartbeatTimeout:  cfg.Cluster.Raft.HeartbeatTimeout.Duration(),
			ElectionTimeout:   cfg.Cluster.Raft.ElectionTimeout.Duration(),
			SnapshotInterval:  cfg.Cluster.Raft.SnapshotInterval.Duration(),
			SnapshotThreshold: cfg.Cluster.Raft.SnapshotThreshold,
		}

		var err error
		raftNode, err = raft.NewNode(raftCfg, store, logger)
		if err != nil {
			logger.Fatal().Err(err).Msg("Failed to initialize Raft")
		}

		// Bootstrap cluster if this is the first node
		if err := raftNode.Bootstrap(); err != nil {
			logger.Warn().Err(err).Msg("Failed to bootstrap Raft cluster (may already be bootstrapped)")
		}

		// Watch for leadership changes
		go func() {
			for isLeader := range raftNode.LeaderCh() {
				sched.SetLeader(isLeader)
			}
		}()
	} else {
		// Single-node mode - always leader
		logger.Info().Msg("Running in single-node mode")
		sched.SetLeader(true)
	}

	// Start scheduler
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := sched.Start(ctx); err != nil {
		logger.Fatal().Err(err).Msg("Failed to start scheduler")
	}

	// Initialize API
	handler := api.NewHandler(store, sched, logger)
	router := api.NewRouter(handler, logger)

	// Start HTTP server
	server := &http.Server{
		Addr:         cfg.Server.HTTP.Address,
		Handler:      router,
		ReadTimeout:  cfg.Server.HTTP.ReadTimeout.Duration(),
		WriteTimeout: cfg.Server.HTTP.WriteTimeout.Duration(),
	}

	go func() {
		logger.Info().Str("address", cfg.Server.HTTP.Address).Msg("Starting HTTP server")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("HTTP server failed")
		}
	}()

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigCh
	logger.Info().Str("signal", sig.String()).Msg("Received shutdown signal")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Stop scheduler
	sched.Stop()

	// Stop Raft
	if raftNode != nil {
		if err := raftNode.Shutdown(); err != nil {
			logger.Error().Err(err).Msg("Raft shutdown failed")
		}
	}

	// Stop HTTP server
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error().Err(err).Msg("HTTP server shutdown failed")
	}

	logger.Info().Msg("Chronos stopped")
}

func setupLogger() zerolog.Logger {
	// Default to JSON logging
	zerolog.TimeFieldFormat = time.RFC3339

	output := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}
	logger := zerolog.New(output).With().Timestamp().Caller().Logger()

	// Set log level from environment
	level := os.Getenv("CHRONOS_LOG_LEVEL")
	switch level {
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "warn":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	return logger
}
