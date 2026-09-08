package cmd

import (
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	u "github.com/Tanq16/senkaimon/utils"
)

var AppVersion = "dev-build"

var debugFlag bool

var rootCmd = &cobra.Command{
	Use:               "senkaimon",
	Short:             "Forward-auth identity service for a Caddy edge",
	Version:           AppVersion,
	CompletionOptions: cobra.CompletionOptions{HiddenDefaultCmd: true},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func setupLogs() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	output := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.DateTime, NoColor: !u.StdoutIsTerminal}
	log.Logger = zerolog.New(output).With().Timestamp().Logger()
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if debugFlag {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
		u.GlobalDebugFlag = true
	}
}

func init() {
	rootCmd.SetHelpCommand(&cobra.Command{Hidden: true})

	rootCmd.PersistentFlags().BoolVar(&debugFlag, "debug", false, "Enable debug logging")
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		return u.ResolveStdin(cmd)
	}

	cobra.OnInitialize(setupLogs)

	rootCmd.AddCommand(setupCmd)
	rootCmd.AddCommand(serveCmd)
}
