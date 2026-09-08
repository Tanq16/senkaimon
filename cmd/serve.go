package cmd

import (
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/Tanq16/senkaimon/internal/audit"
	"github.com/Tanq16/senkaimon/internal/server"
	"github.com/Tanq16/senkaimon/internal/store"
	u "github.com/Tanq16/senkaimon/utils"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Serve the forward-auth endpoint and the management UI",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := store.LoadConfig()
		if err != nil {
			u.PrintFatal("Cannot load the config", err)
		}

		st, err := store.Open(cfg)
		if err != nil {
			u.PrintFatal("Cannot load the state files", err)
		}

		auditPath, err := store.Path(store.AuditFile)
		if err != nil {
			u.PrintFatal("Cannot resolve the config directory", err)
		}
		auditLog, err := audit.Open(auditPath)
		if err != nil {
			u.PrintFatal("Cannot open the audit log", err)
		}
		defer auditLog.Close()

		srv := server.New(cfg, st, auditLog)
		if err := srv.Setup(); err != nil {
			u.PrintFatal("Cannot set up the server", err)
		}

		ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		if err := srv.Run(ctx); err != nil {
			u.PrintFatal("Server error", err)
		}
	},
}
