/*
 * Copyright 2017 Kopano and its licensors
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, version 3,
 * as published by the Free Software Foundation.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	_ "net/http/pprof"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
	"stash.kopano.io/kgol/ksurveyclient-go/autosurvey"
	"stash.kopano.io/kgol/ksurveyclient-go/prometrics"

	cfg "stash.kopano.io/kwm/kwmserver/config"
	"stash.kopano.io/kwm/kwmserver/signaling/server"
	"stash.kopano.io/kwm/kwmserver/version"
)

const defaultListenAddr = "127.0.0.1:8778"

func commandServe() *cobra.Command {
	serveCmd := &cobra.Command{
		Use:   "serve [...args]",
		Short: "Start server and listen for requests",
		Run: func(cmd *cobra.Command, args []string) {
			if err := serve(cmd, args); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	}
	serveCmd.Flags().String("listen", "", fmt.Sprintf("TCP listen address (default \"%s\")", defaultListenAddr))
	serveCmd.Flags().Bool("enable-mcu-api", false, "Enables the MCU API endpoints")
	serveCmd.Flags().Bool("enable-rtm-api", true, "Enables the RPM API endpoints")
	serveCmd.Flags().Bool("enable-www", false, "Enables serving static files")
	serveCmd.Flags().String("www-root", "./www", "Full path for static files to be served when --enable-www is used, defaults to ./www")
	serveCmd.Flags().Bool("enable-docs", false, "Enables serving documentation")
	serveCmd.Flags().String("docs-root", "./docs", "Full path to docs folder to be served when --enable-docs is used, defaults to ./docs")
	serveCmd.Flags().String("admin-tokens-key", "", "Full path to the key file to be used to sign admin tokens")
	serveCmd.Flags().String("iss", "", "OIDC issuer URL")
	serveCmd.Flags().Bool("insecure", false, "Disable TLS certificate and hostname validation")
	serveCmd.Flags().Bool("insecure-auth", false, "Disable verification that auth matches user")
	serveCmd.Flags().StringArray("turn-uri", nil, "TURN uri to send to clients")
	serveCmd.Flags().String("turn-server-shared-secret", "", "Full path to the file which contains the shared secret for TURN server password generation")
	serveCmd.Flags().String("turn-service-url", "", "TURN service API url")
	serveCmd.Flags().String("turn-service-credentials", "", "Full path to the file which contains credentials for the TURN service API (format username:password)")
	serveCmd.Flags().Bool("log-timestamp", true, "Prefix each log line with timestamp")
	serveCmd.Flags().String("log-level", "info", "Log level (one of panic, fatal, error, warn, info or debug)")
	serveCmd.Flags().StringArray("rtm-required-scope", nil, "Require specific scope when checking auth for RTM")
	serveCmd.Flags().Bool("enable-guest-api", false, "Enables the guest API endpoints")
	serveCmd.Flags().Bool("allow-guest-only-channels", false, "If set, guests can join empty channels")
	serveCmd.Flags().String("public-guest-access-regexp", "", "If set, rooms matching this regex can be accessed by guest without invitation (example: ^group/public/.* )")
	serveCmd.Flags().String("registration-conf", "", "Path to a registration.yaml config file")
	serveCmd.Flags().Bool("with-pprof", false, "With pprof enabled")
	serveCmd.Flags().String("pprof-listen", "127.0.0.1:6060", "TCP listen address for pprof")
	serveCmd.Flags().Bool("with-metrics", false, "Enable metrics")
	serveCmd.Flags().String("metrics-listen", "127.0.0.1:6778", "TCP listen address for metrics")

	return serveCmd
}

func serve(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	logTimestamp, _ := cmd.Flags().GetBool("log-timestamp")
	logLevel, _ := cmd.Flags().GetString("log-level")

	logger, err := newLogger(!logTimestamp, logLevel)
	if err != nil {
		return fmt.Errorf("failed to create logger: %v", err)
	}
	logger.Infoln("serve start")

	config := &cfg.Config{
		Logger: logger,

		// Initialize survey client data with operational usage.
		Survey: prometrics.WrapRegistry(autosurvey.DefaultRegistry, map[string]string{
			"rtm_distinct_users_connected_max": "usercnt_active",
			"rtm_group_channels_created_max":   "usercnt_room",
			"rtm_channels_created_max":         "usercnt_equipment",
			"rtm_connections_connected_max":    "usercnt_nonactive",
			"guest_http_logon_success_total":   "usercnt_na_user",
		}),
	}

	listenAddr, _ := cmd.Flags().GetString("listen")
	if listenAddr == "" {
		listenAddr = os.Getenv("KWMSERVERD_LISTEN")
	}
	if listenAddr == "" {
		listenAddr = defaultListenAddr
	}
	config.ListenAddr = listenAddr

	registrationConf, _ := cmd.Flags().GetString("registration-conf")
	if registrationConf != "" {
		config.RegistrationConf, _ = filepath.Abs(registrationConf)
		if _, errStat := os.Stat(config.RegistrationConf); errStat != nil {
			return fmt.Errorf("registration-conf file not found or unable to access: %v", errStat)
		}
	}

	enableGuestAPI, _ := cmd.Flags().GetBool("enable-guest-api")
	config.EnableGuestAPI = enableGuestAPI
	config.GuestsCanCreateChannels, _ = cmd.Flags().GetBool("allow-guest-only-channels")
	config.GuestPublicAccessPattern, _ = cmd.Flags().GetString("public-guest-access-regexp")

	enableMcuAPI, _ := cmd.Flags().GetBool("enable-mcu-api")
	config.EnableMcuAPI = enableMcuAPI

	enableWww, _ := cmd.Flags().GetBool("enable-www")
	config.EnableWww = enableWww
	wwwRoot, _ := cmd.Flags().GetString("www-root")
	if enableWww && wwwRoot != "" {
		wwwRoot, err = filepath.Abs(wwwRoot)
		if err != nil {
			return err
		}
		if stat, errStat := os.Stat(wwwRoot); errStat != nil {
			return fmt.Errorf("unable to access www-root: %v", errStat)
		} else if !stat.IsDir() {
			return fmt.Errorf("www-root must be a directory")
		}
		config.WwwRoot = wwwRoot
	}

	enableDocs, _ := cmd.Flags().GetBool("enable-docs")
	config.EnableDocs = enableDocs
	docsRoot, _ := cmd.Flags().GetString("docs-root")
	if enableDocs && docsRoot != "" {
		docsRoot, err = filepath.Abs(docsRoot)
		if err != nil {
			return err
		}
		if stat, errStat := os.Stat(docsRoot); errStat != nil {
			return fmt.Errorf("unable to access docs-root: %v", errStat)
		} else if !stat.IsDir() {
			return fmt.Errorf("docs-root must be a directory")
		}
		config.DocsRoot = docsRoot
	}

	adminTokensSigningKey, _ := cmd.Flags().GetString("admin-tokens-key")
	if adminTokensSigningKey == "" {
		adminTokensSigningKey = os.Getenv("KWMSERVERD_ADMIN_TOKENS_KEY")
	}
	if adminTokensSigningKey != "" {
		if _, errStat := os.Stat(adminTokensSigningKey); errStat != nil {
			return fmt.Errorf("admin-tokens-key file not found: %v", errStat)
		}
		if f, errOpen := os.Open(adminTokensSigningKey); errOpen == nil {
			var errRead error
			config.AdminTokensSigningKey, errRead = ioutil.ReadAll(f)
			f.Close()
			if errRead != nil {
				return fmt.Errorf("failed to read admin-tokens-key file: %v", errRead)
			}
		} else {
			return fmt.Errorf("failed to open admin-tokens-key file: %v", errOpen)
		}
	}

	if issString, errIf := cmd.Flags().GetString("iss"); errIf == nil && issString != "" {
		config.Iss, errIf = url.Parse(issString)
		if errIf != nil {
			return fmt.Errorf("invalid iss url: %v", errIf)
		}
	}

	turnURIs, _ := cmd.Flags().GetStringArray("turn-uri")
	if len(turnURIs) > 0 {
		// TODO(longsleep): Validate TURN uris.
		config.TURNURIs = turnURIs
	} else {
		turnURIsString := os.Getenv("KWMSERVERD_TURN_URIS")
		if turnURIsString != "" {
			config.TURNURIs = strings.Split(turnURIsString, " ")
		}
	}

	turnServerSharedSecret, _ := cmd.Flags().GetString("turn-server-shared-secret")
	if turnServerSharedSecret == "" {
		turnServerSharedSecret = os.Getenv("KWMSERVERD_TURN_SERVER_SHARED_SECRET")
	}
	if turnServerSharedSecret != "" {
		if _, errStat := os.Stat(turnServerSharedSecret); errStat != nil {
			return fmt.Errorf("turn-server-shared-secret file not found: %v", errStat)
		}
		if f, errOpen := os.Open(turnServerSharedSecret); errOpen == nil {
			ss, errRead := ioutil.ReadAll(f)
			f.Close()
			if errRead != nil {
				return fmt.Errorf("failed to read turn-server-shared-secret file: %v", errRead)
			}
			config.TURNServerSharedSecret = bytes.TrimSpace(ss)
		} else {
			return fmt.Errorf("failed to open turn-server-shared-secret file: %v", errOpen)
		}
	}

	turnServerServiceURL, _ := cmd.Flags().GetString("turn-service-url")
	if turnServerServiceURL == "" {
		turnServerServiceURL = os.Getenv("KWMSERVERD_TURN_SERVER_SERVICE_URL")
	}
	if turnServerServiceURL != "" {
		if u, errURL := url.Parse(turnServerServiceURL); errURL == nil {
			config.TURNServerServiceURL = u.String()
			logger.Infof("using external TURN service: %v", config.TURNServerServiceURL)
		} else {
			return fmt.Errorf("turn-service-url invalid: %v", errURL)
		}
	}

	turnServerServiceCredentials, _ := cmd.Flags().GetString("turn-service-credentials")
	if turnServerServiceCredentials == "" {
		turnServerServiceCredentials = os.Getenv("KWMSERVERD_TURN_SERVER_SERVICE_CREDENTIALS")
	}
	if turnServerServiceCredentials != "" {
		if _, errStat := os.Stat(turnServerServiceCredentials); errStat != nil {
			return fmt.Errorf("turn-service-credentials file not found: %v", errStat)
		}
		if f, errOpen := os.Open(turnServerServiceCredentials); errOpen == nil {
			reader := bufio.NewReader(f)
			credentials, errRead := reader.ReadString('\n')
			f.Close()
			if errRead != nil {
				return fmt.Errorf("failed to read turn-service-credentials file: %v", errRead)
			}
			parts := strings.SplitN(strings.TrimSpace(credentials), ":", 2)
			if len(parts) != 2 {
				return fmt.Errorf("invalid turn-service-credentials format - must be username:password")
			}
			config.TURNServerServiceUsername = parts[0]
			config.TURNServerServicePassword = parts[1]
		} else {
			return fmt.Errorf("failed to open turn-service-credentials file: %v", errOpen)
		}
	}

	var tlsClientConfig *tls.Config
	tlsInsecureSkipVerify, _ := cmd.Flags().GetBool("insecure")
	if tlsInsecureSkipVerify {
		// NOTE(longsleep): This disable http2 client support. See https://github.com/golang/go/issues/14275 for reasons.
		tlsClientConfig = &tls.Config{
			InsecureSkipVerify: tlsInsecureSkipVerify,
		}
		logger.Warnln("insecure mode, TLS client connections are susceptible to man-in-the-middle attacks")
		logger.Debugln("http2 client support is disabled (insecure mode)")
	}
	config.Client = &http.Client{
		Timeout: 60 * time.Second,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
				DualStack: true,
			}).DialContext,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			TLSClientConfig:       tlsClientConfig,
		},
	}

	config.AllowInsecureAuth, _ = cmd.Flags().GetBool("insecure-auth")
	if config.AllowInsecureAuth {
		logger.Warnln("insecure-auth mode, user identifiers are not forced to match auth")
	}

	config.EnableRTMAPI, _ = cmd.Flags().GetBool("enable-rtm-api")
	if config.EnableRTMAPI {
		config.RTMRequiredScopes, _ = cmd.Flags().GetStringArray("rtm-required-scope")
		if len(config.RTMRequiredScopes) > 0 {
			logger.WithField("required_scopes", config.RTMRequiredScopes).Infoln("rtm: access requirements set up")
		}
	}

	// Metrics support.
	config.WithMetrics, _ = cmd.Flags().GetBool("with-metrics")
	metricsListenAddr, _ := cmd.Flags().GetString("metrics-listen")
	if config.WithMetrics && metricsListenAddr != "" {
		reg := prometheus.NewPedanticRegistry()
		config.Metrics = prometheus.WrapRegistererWithPrefix("kwmserver_", reg)
		// Add the standard process and Go metrics to the custom registry.
		reg.MustRegister(
			prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}),
			prometheus.NewGoCollector(),
		)
		go func() {
			metricsListen := metricsListenAddr
			handler := http.NewServeMux()
			logger.WithField("listenAddr", metricsListen).Infoln("metrics enabled, starting listener")
			handler.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
			err := http.ListenAndServe(metricsListen, handler)
			if err != nil {
				logger.WithError(err).Errorln("unable to start metrics listener")
			}
		}()
	}

	srv, err := server.NewServer(config)
	if err != nil {
		return fmt.Errorf("failed to create server: %v", err)
	}

	// Profiling support.
	withPprof, _ := cmd.Flags().GetBool("with-pprof")
	pprofListenAddr, _ := cmd.Flags().GetString("pprof-listen")
	if withPprof && pprofListenAddr != "" {
		runtime.SetMutexProfileFraction(5)
		go func() {
			pprofListen := pprofListenAddr
			logger.WithField("listenAddr", pprofListen).Infoln("pprof enabled, starting listener")
			err := http.ListenAndServe(pprofListen, nil)
			if err != nil {
				logger.WithError(err).Errorln("unable to start pprof listener")
			}
		}()
	}

	// Survey support.
	var guid []byte
	if config.Iss.Hostname() != "localhost" {
		guid = []byte(config.Iss.String())
	}
	err = autosurvey.Start(ctx, "kwmserverd", version.Version, guid)
	if err != nil {
		return fmt.Errorf("failed to start auto survey: %v", err)
	}

	logger.Infoln("serve started")
	return srv.Serve(ctx)
}
