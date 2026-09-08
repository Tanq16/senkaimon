package cmd

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Tanq16/senkaimon/internal/policy"
	"github.com/Tanq16/senkaimon/internal/store"
	u "github.com/Tanq16/senkaimon/utils"
)

var setupFlags struct {
	adminUser     string
	adminPassword string
	adminTOTP     bool
	domain        string
	idpURL        string
	listen        string
	force         bool
}

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Initialize the config directory and the first admin",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		dir, err := store.Dir()
		if err != nil {
			u.PrintFatal("Cannot resolve the config directory", err)
		}
		configPath := filepath.Join(dir, store.ConfigFile)
		if _, err := os.Stat(configPath); err == nil && !setupFlags.force {
			u.PrintFatal(fmt.Sprintf("%s already exists, pass --force to overwrite it", configPath), nil)
		}

		domain := resolveValue(setupFlags.domain, "Domain:", "etherios.work",
			"setup needs --domain, the parent domain the cookie is set on")
		idpURL := resolveValue(setupFlags.idpURL, "IDP URL:", "https://idp."+domain,
			"setup needs --idp-url, the https URL the login page is served from")
		adminUser := resolveValue(setupFlags.adminUser, "Admin username:", "admin",
			"setup needs --admin-user")

		adminPassword := setupFlags.adminPassword
		if adminPassword == "" {
			entered, err := u.PromptPassword("Admin password:")
			if errors.Is(err, u.ErrNoTerminal) {
				u.PrintFatal("setup needs --admin-password, or --admin-password - to read it from stdin", nil)
			}
			if err != nil {
				u.PrintFatal("TUI error", err)
			}
			adminPassword = entered
		}

		cfg := store.DefaultConfig()
		cfg.Server.Listen = setupFlags.listen
		cfg.Identity.IDPURL = idpURL
		cfg.Identity.CookieDomain = store.CookieDomainFor(domain)
		if err := validateIDPDomain(idpURL, domain); err != nil {
			u.PrintFatal("Invalid --idp-url", err)
		}
		if err := cfg.Validate(); err != nil {
			u.PrintFatal("Invalid configuration", err)
		}
		if err := store.ValidateUsername(adminUser); err != nil {
			u.PrintFatal("Invalid --admin-user", err)
		}
		if err := store.ValidatePassword(adminPassword); err != nil {
			u.PrintFatal("Invalid --admin-password", err)
		}

		if err := store.Initialize(); err != nil {
			u.PrintFatal("Cannot create the config directory", err)
		}
		if err := store.WriteConfig(cfg); err != nil {
			u.PrintFatal("Cannot write config.yaml", err)
		}

		st, err := store.Open(cfg)
		if err != nil {
			u.PrintFatal("Cannot open the new state files", err)
		}
		if err := st.CreateUser(adminUser, adminPassword, true, setupFlags.adminTOTP); err != nil {
			u.PrintFatal("Cannot create the admin account", err)
		}
		if err := st.PutPolicy(policy.Policy{
			Name:     "default",
			Subjects: []string{adminUser},
			Rules:    []policy.Rule{{Effect: policy.EffectAllow, Host: "*"}},
		}); err != nil {
			u.PrintFatal("Cannot write the default policy", err)
		}

		u.PrintSuccess("Senkaimon initialized at " + dir)
		u.PrintInfo("Admin account: " + adminUser)
		if setupFlags.adminTOTP {
			u.PrintInfo("TOTP enrolment happens at the first login on " + idpURL + "/login")
		}
		u.PrintInfo("Start it with: senkaimon serve")
	},
}

func resolveValue(flagValue, prompt, placeholder, missing string) string {
	if flagValue != "" {
		return flagValue
	}
	entered, err := u.PromptInput(prompt, placeholder)
	if errors.Is(err, u.ErrNoTerminal) {
		u.PrintFatal(missing, nil)
	}
	if err != nil {
		u.PrintFatal("TUI error", err)
	}
	if entered == "" {
		u.PrintFatal(missing, nil)
	}
	return entered
}

func validateIDPDomain(idpURL, domain string) error {
	parsed, err := url.Parse(idpURL)
	if err != nil {
		return err
	}
	host := strings.ToLower(parsed.Hostname())
	domain = strings.ToLower(strings.TrimPrefix(domain, "."))
	if host != domain && !strings.HasSuffix(host, "."+domain) {
		return fmt.Errorf("host %q is not inside %q, so the cookie set on .%s would never reach the login page", host, domain, domain)
	}
	return nil
}

func init() {
	setupCmd.Flags().StringVar(&setupFlags.adminUser, "admin-user", "", "Username for the first admin account")
	setupCmd.Flags().StringVar(&setupFlags.adminPassword, "admin-password", "", "Password for the first admin account, or - to read it from stdin")
	setupCmd.Flags().BoolVar(&setupFlags.adminTOTP, "admin-totp", true, "Require TOTP for the first admin, enrolled at first login")
	setupCmd.Flags().StringVar(&setupFlags.domain, "domain", "", "Parent domain the session cookie is set on")
	setupCmd.Flags().StringVar(&setupFlags.idpURL, "idp-url", "", "Absolute https URL the login page is served from")
	setupCmd.Flags().StringVar(&setupFlags.listen, "listen", "127.0.0.1:4180", "Address the server binds to")
	setupCmd.Flags().BoolVar(&setupFlags.force, "force", false, "Overwrite an existing config")

	u.MarkStdinLine(setupCmd, "admin-password")
}
