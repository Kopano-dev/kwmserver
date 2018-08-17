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

	"github.com/spf13/cobra"

	"stash.kopano.io/kwm/kwmserver/signaling/server"
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
	serveCmd.Flags().Bool("enable-janus-api", false, "Enables the Janus API endpoints")
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
	serveCmd.Flags().Bool("log-timestamp", true, "Prefix each log line with timestamp")
	serveCmd.Flags().String("log-level", "info", "Log level (one of panic, fatal, error, warn, info or debug)")

	// Pprof support.
	serveCmd.Flags().Bool("with-pprof", false, "With pprof enabled")
	serveCmd.Flags().String("pprof-listen", "127.0.0.1:6060", "TCP listen address for pprof")

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

	config := &server.Config{
		Logger: logger,
	}

	listenAddr, _ := cmd.Flags().GetString("listen")
	if listenAddr == "" {
		listenAddr = os.Getenv("KWMSERVERD_LISTEN")
	}
	if listenAddr == "" {
		listenAddr = defaultListenAddr
	}
	config.ListenAddr = listenAddr

	enableMcuAPI, _ := cmd.Flags().GetBool("enable-mcu-api")
	config.EnableMcuAPI = enableMcuAPI

	enableJanusAPI, _ := cmd.Flags().GetBool("enable-janus-api")
	config.EnableJanusAPI = enableJanusAPI

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
			var errRead error
			config.TURNServerSharedSecret, errRead = ioutil.ReadAll(f)
			f.Close()
			if errRead != nil {
				return fmt.Errorf("failed to read turn-server-shared-secret file: %v", errRead)
			}
		} else {
			return fmt.Errorf("failed to open turn-server-shared-secret file: %v", errOpen)
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

	logger.Infoln("serve started")
	return srv.Serve(ctx)
}
