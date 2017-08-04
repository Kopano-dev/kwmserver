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

	"stash.kopano.io/kwm/kwmserver/signaling"
	kwmAPIv1Service "stash.kopano.io/kwm/kwmserver/signaling/api-v1/service"
	kwmJanusService "stash.kopano.io/kwm/kwmserver/signaling/janus/service"
)

// Server is our HTTP server implementation.
type Server struct {
	listenAddr string
	logger     logrus.FieldLogger
}

// NewServer constructs a server from the provided parameters.
func NewServer(c *Config) (*Server, error) {
	s := &Server{
		listenAddr: c.ListenAddr,
		logger:     c.Logger,
	}

	return s, nil
}

// WithMetrics adds metrics logging to the provided http.Handler. When the
// handler is done, the context is cancelled, logging metrics.
func (s *Server) WithMetrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		// Create per request cancel context.
		ctx, cancel := context.WithCancel(req.Context())
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
		// Run the request.
		next.ServeHTTP(loggedWriter, req.WithContext(ctx))
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
func (s *Server) AddRoutes(ctx context.Context, router *mux.Router, services []signaling.Service) http.Handler {
	// TODO(longsleep): Add subpath support to all handlers and paths.
	router.Handle("/health-check", s.WithMetrics(http.HandlerFunc(s.HealthCheckHandler)))

	for _, service := range services {
		service.AddRoutes(ctx, router, s.WithMetrics)
	}

	return router
}

// Serve starts all the accociated servers resources and listeners and blocks
// forever until signals or error occurs. Returns error and gracefully stops
// all HTTP listeners before return.
func (s *Server) Serve(ctx context.Context) error {
	serveCtx, serveCtxCancel := context.WithCancel(ctx)
	defer serveCtxCancel()

	logger := s.logger
	apiv1Service := kwmAPIv1Service.NewHTTPService(serveCtx, logger, nil)
	janusService := kwmJanusService.NewHTTPService(serveCtx, logger, apiv1Service.MCUM())
	services := []signaling.Service{
		apiv1Service,
		janusService,
	}

	errCh := make(chan error, 2)
	exitCh := make(chan bool, 1)
	signalCh := make(chan os.Signal)

	router := mux.NewRouter()
	s.AddRoutes(serveCtx, router, services)

	// HTTP listener.
	srv := &http.Server{
		Handler: s.AddContext(serveCtx, router),
	}

	logger.WithField("listenAddr", s.listenAddr).Infoln("starting http listener")
	listener, err := net.Listen("tcp", s.listenAddr)
	if err != nil {
		return err
	}
	logger.Infoln("ready to handle requests")

	go func() {
		serveErr := srv.Serve(listener)
		if serveErr != nil {
			errCh <- serveErr
		}

		logger.Debugln("http listener stopped")
		close(exitCh)
	}()

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
			numActive := apiv1Service.NumActive()
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
