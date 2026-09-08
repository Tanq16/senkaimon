package cmd

import (
	"os/signal"
	"syscall"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/Tanq16/senkaimon/internal/audit"
	"github.com/Tanq16/senkaimon/internal/server"
	"github.com/Tanq16/senkaimon/internal/store"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Serve the forward-auth endpoint and the management UI",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := store.LoadConfig()
		if err != nil {
			log.Fatal().Err(err).Msg("failed to load config")
		}

		st, err := store.Open(cfg)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to load state")
		}

		auditPath, err := store.Path(store.AuditFile)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to resolve the config directory")
		}
		auditLog, err := audit.Open(auditPath)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to open the audit log")
		}
		defer auditLog.Close()

		srv := server.New(cfg, st, auditLog)
		if err := srv.Setup(); err != nil {
			log.Fatal().Err(err).Msg("failed to set up the server")
		}

		ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		if err := srv.Run(ctx); err != nil {
			log.Fatal().Err(err).Msg("server error")
		}
	},
}
