// +build supportedBuild

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"stash.kopano.io/kc/libkustomer/lib/kustomer/libkustomer"

	"stash.kopano.io/kwm/kwmserver/signaling/server"
	"stash.kopano.io/kwm/kwmserver/version"
)

var supportedBuildTag = "libkustomer" + version.KustomerBuildVersion

func initializeBuild(ctx context.Context, logger logrus.FieldLogger) error {
	// Initialize libkustomer before startup.
	return kustomerServeHook(ctx, logger)
}

var productUserAgent string = "Kopano Webmeetings Server/" + version.Version
var kustomerInitializeTimeout = time.Duration(60) * time.Second

func kustomerServeHook(ctx context.Context, logger logrus.FieldLogger) error {
	logger.Infoln("KUSTOMER initializing")

	libkustomer.Init(&libkustomer.InitOptions{
		ProductUserAgent:   &productUserAgent,
		AutoRefresh:        true,
		DefaultDebugLogger: logger,
	})

	kustomerCtx, kustomerCtxCancel := context.WithCancel(ctx)

	err := libkustomer.Initialize(kustomerCtx, &server.KustomerProductName)
	if err != nil {
		return fmt.Errorf("KUSTOMER initialization failed: %w", err)
	}

	if kustomerInitializeTimeout > 0 {
		err = func() error {
			logger.Debugf("KUSTOMER waiting on initialization for %s", kustomerInitializeTimeout)
			errCh := make(chan error, 1)
			signalCh := make(chan os.Signal)

			go func() {
				errCh <- libkustomer.WaitUntilReady(kustomerInitializeTimeout)
			}()
			// Wait for signal or result.
			signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)
			defer signal.Reset()
			for {
				select {
				case err = <-errCh:
					return err
				case reason := <-signalCh:
					logger.WithField("signal", reason).Warnln("received signal")
					kustomerCtxCancel()
					// breaks
				}
			}
		}()
		if err != nil {
			return fmt.Errorf("KUSTOMER failed to initialize: %w", err)
		}
		logger.Infoln("KUSTOMER initialized")
	}

	return nil
}
