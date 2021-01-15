// +build supportedBuild

package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	kustomer "stash.kopano.io/kc/libkustomer"
	"stash.kopano.io/kc/libkustomer/lib/kustomer/libkustomer"
	apiv1 "stash.kopano.io/kwm/kwmserver/signaling/api-v1"
	"stash.kopano.io/kwm/kwmserver/signaling/rtm"
)

func bootstrapBuild(ctx context.Context, s *Server) error {
	// Bootstrap libkustomer for operation.
	return kustomerBootstrap(ctx, s)
}

// KustomerProductName is the product name to use for license checks.
var KustomerProductName string = "meet"

func kustomerBootstrap(ctx context.Context, s *Server) error {
	currentClaimsFetcher := func() (*kustomer.KopanoProductClaims, error) {
		kpc, err := libkustomer.CurrentKopanoProductClaims()
		if err != nil {
			s.logger.WithError(err).Errorln("KUSTOMER failed to get current claims")
		}

		return kpc, err
	}

	metricsLicenseChecker := func(kpc *kustomer.KopanoProductClaims) error {
		if s.config.Gatherer != nil {
			gathered, err := s.config.Gatherer.Gather()
			if err != nil {
				return fmt.Errorf("KUSTOMER metrics license check error: %w", err)
			}
			for _, mf := range gathered {
				if mf.Name != nil && *mf.Name == "kwmserver_rtm_group_channels_created_current" {
					metrics := mf.GetMetric()
					if len(metrics) > 0 {
						metric := metrics[0]
						gauge := metric.GetGauge()
						if gauge != nil {
							if err := kpc.EnsureFloat64WithOperator(KustomerProductName, "max-groups", gauge.GetValue(), kustomer.OperatorGreaterThanOrEqual); err != nil {
								s.logger.WithError(err).Warnf("KUSTOMER max-groups overuse, %.0f active groups", gauge.GetValue())
								return err
							}
						}
					}
				}
			}
		}
		return nil
	}

	localLicenseChecker := func(kpc *kustomer.KopanoProductClaims) error {
		var res error

		if err := kpc.EnsureOK(KustomerProductName); err != nil {
			s.logger.Errorf("KUSTOMER was unable to find a '%s' license: %s", KustomerProductName, err)
			res = err
		}

		if s.config.EnableGuestAPI {
			if err := kpc.EnsureBool(KustomerProductName, "guests", true); err != nil {
				s.logger.WithError(err).Warnln("KUSTOMER guest API support not licensed")
				res = err
			}
		}
		if s.config.EnableMcuAPI {
			if err := kpc.EnsureBool(KustomerProductName, "sfu", true); err != nil {
				s.logger.WithError(err).Warnln("KUSTOMER SFU/MCU support not licensed")
				res = err
			}
		}
		if s.config.TURNServerServiceUsername != "" {
			if err := kpc.EnsureBool(KustomerProductName, "turnaccess", true); err != nil {
				s.logger.WithError(err).Warnln("KUSTOMER SFU/MCU support not licensed")
				res = err
			}
		}

		if err := metricsLicenseChecker(kpc); err != nil {
			res = err
		}

		if res != nil {
			return fmt.Errorf("KUSTOMER local license check issue: %w", res)
		}
		return nil
	}

	ensuredClaimsLogger := func(kpc *kustomer.KopanoProductClaims) {
		data, _ := json.Marshal(kpc.Dump())
		s.logger.WithField("claims", string(data)).Println("KUSTOMER ensured claims")
	}

	// Fetch current claims (this is blocking)
	kpc, err := currentClaimsFetcher()
	if err == nil {
		ensuredClaimsLogger(kpc)
	}

	// Initialize and run license check, as soon as services are ready.
	go func() {
		s.services.Wait()

		rtmm := s.services.RTMManager.(*rtm.Manager)
		var kustomerStatus uint64
		var kustomerStatusMutex sync.Mutex
		var unknownError uint64 = 1

		reporter := func(checkErr error) error {
			kustomerStatusMutex.Lock()
			defer kustomerStatusMutex.Unlock()
			var changed bool
			if checkErr != nil {
				var numericErr kustomer.ErrNumeric
				if errors.As(checkErr, &numericErr) {
					if uint64(numericErr) != kustomerStatus {
						if kustomerStatus != uint64(numericErr) {
							kustomerStatus = uint64(numericErr)
							changed = true
						}
					}
				} else {
					if kustomerStatus != unknownError {
						kustomerStatus = unknownError
						changed = true
					}
				}
			} else {
				if kustomerStatus != 0 {
					kustomerStatus = 0
					changed = true
				}
			}
			if changed {
				if kustomerStatus == 0 {
					s.logger.Warnln("KUSTOMER license is OK")
				} else {
					s.logger.Warnf("KUSTOMER license issues detected (:0x%x)", kustomerStatus)
				}
				cks := kustomerStatus
				rtmm.OnServerStatus(&apiv1.ServerStatus{ // TODO(longsleep): Move server status to server struct.
					Kustomer: &cks,
				})
			}
			return checkErr
		}

		err := libkustomer.SetNotifyWhenUpdated(func() {
			s.logger.Infoln("KUSTOMER ensured claim notify received")

			if kpc, err := currentClaimsFetcher(); err == nil {
				go func() {
					ensuredClaimsLogger(kpc)
					reporter(localLicenseChecker(kpc))
				}()
			}
		}, func() {
			s.logger.Infoln("KUSTOMER ensured claim notify watch exit")
		})
		if err != nil {
			panic(fmt.Errorf("KUSTOMER failed to watch for updates: %w", err))
		}

		// Trigger initial check on startup.
		reporter(localLicenseChecker(kpc))

		// Start background metrics license checker.
		go func() {
			var metricsStatus bool
			for {
				select {
				case <-ctx.Done():
					return
				case <-time.After(1 * time.Minute):
				}
				if kpc, err := currentClaimsFetcher(); err == nil {
					checkErr := metricsLicenseChecker(kpc)
					if checkErr != nil {
						metricsStatus = true
						reporter(checkErr)
					} else {
						if metricsStatus == true {
							reporter(localLicenseChecker(kpc))
							metricsStatus = false
						}
					}
				}
			}
		}()
	}()

	return nil
}
