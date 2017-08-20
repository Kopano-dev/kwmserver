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
	"fmt"
	_ "net/http/pprof"
	"os"

	"github.com/spf13/cobra"

	"stash.kopano.io/kwm/kwmserver/signaling/server"
)

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
	serveCmd.Flags().String("listen", "127.0.0.1:8778", "TCP listen address")
	serveCmd.Flags().Bool("enable-mcu-api", false, "Enables the MCU API endpoints")
	serveCmd.Flags().Bool("enable-janus-api", false, "Enables the Janus API endpoints")
	serveCmd.Flags().Bool("with-pprof", false, "With pprof enabled")
	serveCmd.Flags().String("pprof-listen", "127.0.0.1:6060", "TCP listen address for pprof")

	return serveCmd
}

func serve(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	logger, err := newLogger()
	if err != nil {
		return fmt.Errorf("failed to create logger: %v", err)
	}
	logger.Infoln("serve start")

	config := &server.Config{
		Logger: logger,
	}

	listenAddr, _ := cmd.Flags().GetString("listen")
	config.ListenAddr = listenAddr
	enableMcuAPI, _ := cmd.Flags().GetBool("enable-mcu-api")
	config.EnableMcuAPI = enableMcuAPI
	enableJanusAPI, _ := cmd.Flags().GetBool("enable-janus-api")
	config.EnableJanusAPI = enableJanusAPI
	withPprof, _ := cmd.Flags().GetBool("with-pprof")
	config.WithPprof = withPprof
	pprofListenAddr, _ := cmd.Flags().GetString("pprof-listen")
	config.PprofListenAddr = pprofListenAddr

	srv, err := server.NewServer(config)
	if err != nil {
		return fmt.Errorf("failed to create server: %v", err)
	}

	logger.Infoln("serve started")
	return srv.Serve(ctx)
}
