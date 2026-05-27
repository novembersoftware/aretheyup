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
	probeSweepInterval   = 5 * time.Second
	probeCleanupInterval = time.Hour
	probeRawRetention    = 30 * 24 * time.Hour
	probeClaimBatchSize  = 16
	probeRegionGlobal    = "global"
	probeUserAgent       = "aretheyup-probe/1.0"
)

type probeStore interface {
	ClaimDueProbeConfigs(ctx context.Context, now time.Time, limit int) ([]structs.ProbeConfig, error)
	CompleteProbeLease(ctx context.Context, configID uint, leaseToken string, result structs.ProbeResult, checkedAt time.Time) error
	DeleteProbeResultsOlderThan(ctx context.Context, cutoff time.Time) (int64, error)
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
	if err := cleanupOldProbeResults(ctx, store, time.Now().UTC()); err != nil {
		log.Error().Err(err).Msg("Failed to clean old probe results")
	}

	sweepTicker := time.NewTicker(probeSweepInterval)
	defer sweepTicker.Stop()

	cleanupTicker := time.NewTicker(probeCleanupInterval)
	defer cleanupTicker.Stop()

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
		case <-cleanupTicker.C:
			if err := cleanupOldProbeResults(ctx, store, time.Now().UTC()); err != nil {
				log.Error().Err(err).Msg("Failed to clean old probe results")
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
			checkedAt := time.Now().UTC()
			result, err := executeClaimedProbe(ctx, executor, cfg)
			if err != nil {
				log.Error().
					Err(err).
					Uint("probe_config_id", cfg.ID).
					Uint("service_id", cfg.ServiceID).
					Msg("Probe execution failed unexpectedly")
				continue
			}

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

func cleanupOldProbeResults(ctx context.Context, store probeStore, now time.Time) error {
	cutoff := now.Add(-probeRawRetention)
	deleted, err := store.DeleteProbeResultsOlderThan(ctx, cutoff)
	if err != nil {
		return err
	}
	if deleted > 0 {
		log.Info().Int64("deleted", deleted).Time("cutoff", cutoff).Msg("Deleted expired probe results")
	}

	return nil
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
	expectedStatus := normalizedExpectedStatus(cfg.ExpectedStatus)

	result := structs.ProbeResult{
		Region:         probeRegionGlobal,
		Success:        statusCode == expectedStatus,
		StatusCode:     &statusCode,
		ResponseTimeMs: &responseTimeMs,
	}
	if !result.Success {
		result.FailureType = structs.ProbeFailureTypeHTTPStatus
		result.ErrorMessage = fmt.Sprintf("unexpected status: got %d want %d", statusCode, expectedStatus)
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

func normalizedProbeTimeout(timeoutSeconds int) int {
	if timeoutSeconds <= 0 {
		return 10
	}
	return timeoutSeconds
}
