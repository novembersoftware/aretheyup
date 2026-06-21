package workers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/novembersoftware/aretheyup/storage"
	"github.com/novembersoftware/aretheyup/structs"
	"github.com/rs/zerolog/log"
)

const (
	probeSweepInterval  = 5 * time.Second
	probeClaimBatchSize = 16
	probeRegionGlobal   = "global"
	probeUserAgent      = "aretheyup-probe/1.0"
)

type probeStore interface {
	ClaimDueProbeConfigs(ctx context.Context, now time.Time, limit int) ([]structs.ProbeConfig, error)
	CompleteProbeLease(ctx context.Context, configID uint, leaseToken string, result structs.ProbeResult, checkedAt time.Time) error
}

type probeExecutor interface {
	Execute(ctx context.Context, cfg structs.ProbeConfig) (structs.ProbeResult, error)
}

type httpProbeExecutor struct {
	client *http.Client
}

func RunProbeWorker(store *storage.Storage) error {
	return runProbeWorker(context.Background(), store, &httpProbeExecutor{
		client: &http.Client{},
	})
}

func runProbeWorker(ctx context.Context, store probeStore, executor probeExecutor) error {
	sweepTicker := time.NewTicker(probeSweepInterval)
	defer sweepTicker.Stop()

	if err := runProbeSweep(ctx, store, executor, time.Now().UTC()); err != nil {
		log.Error().Err(err).Msg("Probe sweep failed")
	}

	for {
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.Canceled) {
				return nil
			}
			return ctx.Err()
		case <-sweepTicker.C:
			if err := runProbeSweep(ctx, store, executor, time.Now().UTC()); err != nil {
				log.Error().Err(err).Msg("Probe sweep failed")
			}
		}
	}
}

func runProbeSweep(ctx context.Context, store probeStore, executor probeExecutor, now time.Time) error {
	for {
		claimed, err := store.ClaimDueProbeConfigs(ctx, now, probeClaimBatchSize)
		if err != nil {
			return err
		}
		if len(claimed) == 0 {
			return nil
		}

		for _, cfg := range claimed {
			result, err := executeClaimedProbe(ctx, executor, cfg)
			if err != nil {
				log.Error().
					Err(err).
					Uint("probe_config_id", cfg.ID).
					Uint("service_id", cfg.ServiceID).
					Msg("Probe execution failed unexpectedly")
				continue
			}

			checkedAt := time.Now().UTC()
			if err := store.CompleteProbeLease(ctx, cfg.ID, cfg.LeaseToken, result, checkedAt); err != nil {
				log.Error().
					Err(err).
					Uint("probe_config_id", cfg.ID).
					Uint("service_id", cfg.ServiceID).
					Msg("Failed to finalize probe result")
				continue
			}

			log.Debug().
				Uint("service_id", cfg.ServiceID).
				Uint("probe_config_id", cfg.ID).
				Bool("success", result.Success).
				Str("url", cfg.URL).
				Msg("Probe completed")
		}

		if len(claimed) < probeClaimBatchSize {
			return nil
		}
	}
}

func executeClaimedProbe(ctx context.Context, executor probeExecutor, cfg structs.ProbeConfig) (structs.ProbeResult, error) {
	execCtx, cancel := context.WithTimeout(ctx, time.Duration(normalizedProbeTimeout(cfg.TimeoutSeconds))*time.Second)
	defer cancel()

	return executor.Execute(execCtx, cfg)
}

func (e *httpProbeExecutor) Execute(ctx context.Context, cfg structs.ProbeConfig) (structs.ProbeResult, error) {
	start := time.Now()

	req, err := http.NewRequestWithContext(ctx, normalizedProbeMethod(cfg.Method), cfg.URL, nil)
	if err != nil {
		return structs.ProbeResult{
			FailureType:  classifyInvalidProbeRequest(err),
			Region:       probeRegionGlobal,
			Success:      false,
			ErrorMessage: err.Error(),
		}, nil
	}
	req.Header.Set("User-Agent", probeUserAgent)

	resp, err := e.client.Do(req)
	if err != nil {
		return structs.ProbeResult{
			FailureType:  classifyProbeFailure(err),
			Region:       probeRegionGlobal,
			Success:      false,
			ErrorMessage: err.Error(),
		}, nil
	}
	defer resp.Body.Close()

	statusCode := resp.StatusCode
	responseTimeMs := int(time.Since(start).Milliseconds())

	result := structs.ProbeResult{
		Region:         probeRegionGlobal,
		Success:        isFunctionalProbeStatus(statusCode, cfg.ExpectedStatus),
		StatusCode:     &statusCode,
		ResponseTimeMs: &responseTimeMs,
	}
	if !result.Success {
		result.FailureType = structs.ProbeFailureTypeHTTPStatus
		result.ErrorMessage = fmt.Sprintf("unhealthy status: got %d", statusCode)
	}

	return result, nil
}

func normalizedProbeMethod(method string) string {
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		return http.MethodGet
	}
	return method
}

func normalizedExpectedStatus(status int) int {
	if status <= 0 {
		return http.StatusOK
	}
	return status
}

func isFunctionalProbeStatus(statusCode, expectedStatus int) bool {
	if statusCode == normalizedExpectedStatus(expectedStatus) {
		return true
	}

	// A reachable app that returns auth, rate-limit, or other client responses is
	// still functionally up for our probe purposes. Reserve failures for upstream
	// server errors and transport-level problems.
	return statusCode >= 400 && statusCode < 500
}

func normalizedProbeTimeout(timeoutSeconds int) int {
	if timeoutSeconds <= 0 {
		return 10
	}
	return timeoutSeconds
}
