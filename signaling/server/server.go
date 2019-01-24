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

package server

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/longsleep/go-metrics/loggedwriter"
	"github.com/longsleep/go-metrics/timing"
	"github.com/sirupsen/logrus"
	kcoidc "stash.kopano.io/kc/libkcoidc"
	"stash.kopano.io/kgol/rndm"

	"stash.kopano.io/kwm/kwmserver/signaling"
	"stash.kopano.io/kwm/kwmserver/signaling/admin"
	apiv1 "stash.kopano.io/kwm/kwmserver/signaling/api-v1/service"
	apiv2 "stash.kopano.io/kwm/kwmserver/signaling/api-v2/service"
	"stash.kopano.io/kwm/kwmserver/signaling/guest"
	"stash.kopano.io/kwm/kwmserver/signaling/mcu"
	"stash.kopano.io/kwm/kwmserver/signaling/rtm"
	"stash.kopano.io/kwm/kwmserver/signaling/www"
	"stash.kopano.io/kwm/kwmserver/turn"
)

// Server is our HTTP server implementation.
type Server struct {
	config *Config

	listenAddr string
	logger     logrus.FieldLogger

	requestLog bool
}

// NewServer constructs a server from the provided parameters.
func NewServer(c *Config) (*Server, error) {
	s := &Server{
		config: c,

		listenAddr: c.ListenAddr,
		logger:     c.Logger,

		requestLog: os.Getenv("KOPANO_DEBUG_SERVER_REQUEST_LOG") == "1",
	}

	return s, nil
}

// WithMetrics adds metrics logging to the provided http.Handler. When the
// handler is done, the context is cancelled, logging metrics.
func (s *Server) WithMetrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		// Create per request cancel context.
		ctx, cancel := context.WithCancel(req.Context())

		if s.requestLog {
			loggedWriter := metrics.NewLoggedResponseWriter(rw)
			// Create per request context.
			ctx = timing.NewContext(ctx, func(duration time.Duration) {
				// This is the stop callback, called when complete with duration.
				durationMs := float64(duration) / float64(time.Millisecond)
				// Log request.
				s.logger.WithFields(logrus.Fields{
					"status":     loggedWriter.Status(),
					"method":     req.Method,
					"path":       req.URL.Path,
					"remote":     req.RemoteAddr,
					"duration":   durationMs,
					"referer":    req.Referer(),
					"user-agent": req.UserAgent(),
					"origin":     req.Header.Get("Origin"),
				}).Debug("HTTP request complete")
			})
			rw = loggedWriter
		}

		// Run the request.
		next.ServeHTTP(rw, req.WithContext(ctx))

		// Cancel per request context when done.
		cancel()
	})
}

// AddContext adds the accociated server's context to the provided http.Hander
// request.
func (s *Server) AddContext(parent context.Context, next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		next.ServeHTTP(rw, req.WithContext(parent))
	})
}

// AddRoutes add the accociated Servers URL routes to the provided router with
// the provided context.Context.
func (s *Server) AddRoutes(ctx context.Context, router *mux.Router) http.Handler {
	// TODO(longsleep): Add subpath support to all handlers and paths.
	router.Handle("/health-check", s.WithMetrics(http.HandlerFunc(s.HealthCheckHandler)))

	return router
}

// Serve starts all the accociated servers resources and listeners and blocks
// forever until signals or error occurs. Returns error and gracefully stops
// all HTTP listeners before return.
func (s *Server) Serve(ctx context.Context) error {
	var err error

	serveCtx, serveCtxCancel := context.WithCancel(ctx)
	defer serveCtxCancel()

	logger := s.logger
	services := &signaling.Services{}

	// OpenID connect.
	var oidcp *kcoidc.Provider
	if s.config.Iss != nil {
		oidcp, err = kcoidc.NewProvider(s.config.Client, log.New(os.Stderr, "kcoidc: ", log.LstdFlags), os.Getenv("KCOIDC_DEBUG") == "1")
		if err != nil {
			return fmt.Errorf("failed to create kcoidc provider for server: %v", err)
		}
		err = oidcp.Initialize(serveCtx, s.config.Iss)
		if err != nil {
			return fmt.Errorf("OIDC provider initialization error: %v", err)
		}
		if errOIDCInitialize := oidcp.WaitUntilReady(serveCtx, 10*time.Second); errOIDCInitialize != nil {
			// NOTE(longsleep): Do not treat this as error - just log.
			logger.WithError(errOIDCInitialize).WithField("iss", s.config.Iss).Warnf("failed to initialize OIDC provider")
		} else {
			logger.WithField("iss", s.config.Iss).Debugln("OIDC provider initialized")
		}
	}

	// TURN credentials support.
	var turnsrv turn.Server
	if s.config.TURNServerSharedSecret != nil {
		if len(s.config.TURNURIs) == 0 {
			return fmt.Errorf("at least one turn-uri is required but none given")
		}

		turnsrv, err = turn.NewSharedsecretServer(
			s.config.TURNURIs,
			s.config.TURNServerSharedSecret,
			0,
		)
		if err != nil {
			return fmt.Errorf("failed to initialize TURN server support: %v", err)
		}

		logger.WithField("uris", s.config.TURNURIs).Debugln("TURN credentials support enabled")
	} else if s.config.TURNServerServiceUsername != "" {
		if s.config.TURNServerServiceURL == "" {
			return fmt.Errorf("TURN server service URL cannot be empty")
		}

		turnsrv, err = turn.NewServerAuthServer(
			s.config.TURNServerServiceURL,
			s.config.TURNServerServiceUsername,
			s.config.TURNServerServicePassword,
			s.config.Client,
		)
		if err != nil {
			return fmt.Errorf("failed to initialize TURN service support: %v", err)
		}
	}

	// Admin API.
	adminm := admin.NewManager(serveCtx, "", logger)
	if s.config.AdminTokensSigningKey == nil || len(s.config.AdminTokensSigningKey) < 32 {
		s.config.AdminTokensSigningKey = make([]byte, 32)
		if _, err = rndm.ReadRandomBytes(s.config.AdminTokensSigningKey); err != nil {
			return fmt.Errorf("unable to create random key, %v", err)
		}
		logger.Warnln("admin: using random admin tokens singing key - API endpoint admin disabled")
	} else {
		// Only expose admin API when a key was set.
		services.AdminManager = adminm
		logger.Infoln("admin: API endpoint enabled")
	}
	adminm.AddTokenKey("", s.config.AdminTokensSigningKey)

	// MCU API.
	var mcum *mcu.Manager
	if s.config.EnableMcuAPI {
		mcum = mcu.NewManager(serveCtx, "", logger)
		services.MCUManager = mcum
		logger.Infoln("mcu: API endpoint enabled")
	}

	// Guest API.
	var guestm *guest.Manager
	if true {
		guestm = guest.NewManager(serveCtx, "", logger)
		services.GuestManager = guestm
		logger.Infoln("guest: API endpoint enabled")
	}

	// RTM API.
	var rtmm *rtm.Manager
	if s.config.EnableRTMAPI {
		rtmm = rtm.NewManager(serveCtx, "", s.config.AllowInsecureAuth, s.config.RTMRequiredScopes, logger, mcum, adminm, oidcp, turnsrv)
		services.RTMManager = rtmm
		logger.Infoln("rtm: API endpoint enabled")
	}

	// HTTP services.
	router := mux.NewRouter()
	httpServices := []signaling.Service{}

	// Basic routes provided by server.
	s.AddRoutes(ctx, router)

	if true {
		apiv1Service := apiv1.NewHTTPService(serveCtx, logger, services)
		apiv1Service.AddRoutes(ctx, router, s.WithMetrics)
		httpServices = append(httpServices, apiv1Service)
	}

	if true {
		apiv2Service := apiv2.NewHTTPService(serveCtx, logger, services)
		apiv2Service.AddRoutes(ctx, router, s.WithMetrics)
		httpServices = append(httpServices, apiv2Service)
	}

	if s.config.EnableDocs {
		if s.config.DocsRoot == "" {
			return fmt.Errorf("unable to enable docs API without docs root")
		}
		docsService := www.NewHTTPService(serveCtx, logger, "/docs", s.config.DocsRoot)
		docsService.AddRoutes(ctx, router, s.WithMetrics)
		httpServices = append(httpServices, docsService)
		logger.Infof("docs: endpoints from %s enabled", s.config.DocsRoot)
	}

	if s.config.EnableWww {
		if s.config.WwwRoot == "" {
			return fmt.Errorf("unable to enable www API without www root")
		}
		wwwService := www.NewHTTPService(serveCtx, logger, "/", s.config.WwwRoot)
		wwwService.AddRoutes(ctx, router, s.WithMetrics)
		httpServices = append(httpServices, wwwService)
		logger.Infof("www: endpoints from %s enabled", s.config.WwwRoot)
	}

	errCh := make(chan error, 2)
	exitCh := make(chan bool, 1)
	signalCh := make(chan os.Signal)

	// HTTP listener.
	logger.WithField("listenAddr", s.listenAddr).Infoln("starting http listener")
	listener, err := net.Listen("tcp", s.listenAddr)
	if err != nil {
		return err
	}
	srv := &http.Server{
		Handler: s.AddContext(serveCtx, router),
	}
	go func() {
		serveErr := srv.Serve(listener)
		if serveErr != nil {
			errCh <- serveErr
		}

		logger.Debugln("http listener stopped")
		close(exitCh)
	}()
	logger.Infoln("ready to handle requests")

	// Wait for exit or error.
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)
	select {
	case err = <-errCh:
		// breaks
	case reason := <-signalCh:
		logger.WithField("signal", reason).Warnln("received signal")
		// breaks
	}

	// Shutdown, server will stop to accept new connections, requires Go 1.8+.
	logger.Infoln("clean server shutdown start")
	shutDownCtx, shutDownCtxCancel := context.WithTimeout(ctx, 10*time.Second)
	if shutdownErr := srv.Shutdown(shutDownCtx); shutdownErr != nil {
		logger.WithError(shutdownErr).Warn("clean server shutdown failed")
	}

	// Cancel our own context, wait on managers.
	serveCtxCancel()
	func() {
		for {
			var numActive uint64
			for _, service := range httpServices {
				numActive = numActive + service.NumActive()
			}
			if numActive == 0 {
				select {
				case <-exitCh:
					return
				default:
					// HTTP listener has not quit yet.
					logger.Info("waiting for http listener to exit")
				}
			} else {
				logger.WithField("connections", numActive).Info("waiting for active connections to exit")
			}
			select {
			case reason := <-signalCh:
				logger.WithField("signal", reason).Warn("received signal")
				return
			case <-time.After(100 * time.Millisecond):
			}
		}
	}()
	shutDownCtxCancel() // prevent leak.

	return err
}
