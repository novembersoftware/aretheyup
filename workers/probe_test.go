package workers

import (
	"context"
	"crypto/x509"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/novembersoftware/aretheyup/structs"
)

type fakeProbeStore struct {
	mu        sync.Mutex
	claimed   [][]structs.ProbeConfig
	completed []struct {
		configID  uint
		token     string
		result    structs.ProbeResult
		checkedAt time.Time
	}
	cleanupCalls []struct {
		successCutoff time.Time
		failureCutoff time.Time
		batchSize     int
		maxBatches    int
	}
	deleteRows    []int64
	deleteBatches []int
	vacuumCalls   int
}

func (s *fakeProbeStore) ClaimDueProbeConfigs(_ context.Context, _ time.Time, _ int) ([]structs.ProbeConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.claimed) == 0 {
		return nil, nil
	}

	next := s.claimed[0]
	s.claimed = s.claimed[1:]
	return next, nil
}

func (s *fakeProbeStore) CompleteProbeLease(_ context.Context, configID uint, leaseToken string, result structs.ProbeResult, checkedAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.completed = append(s.completed, struct {
		configID  uint
		token     string
		result    structs.ProbeResult
		checkedAt time.Time
	}{
		configID:  configID,
		token:     leaseToken,
		result:    result,
		checkedAt: checkedAt,
	})
	return nil
}

func (s *fakeProbeStore) DeleteExpiredRawProbeResults(_ context.Context, successCutoff, failureCutoff time.Time, batchSize, maxBatches int) (int64, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupCalls = append(s.cleanupCalls, struct {
		successCutoff time.Time
		failureCutoff time.Time
		batchSize     int
		maxBatches    int
	}{
		successCutoff: successCutoff,
		failureCutoff: failureCutoff,
		batchSize:     batchSize,
		maxBatches:    maxBatches,
	})

	callIndex := len(s.cleanupCalls) - 1
	deleted := int64(3)
	if callIndex < len(s.deleteRows) {
		deleted = s.deleteRows[callIndex]
	}
	batches := 1
	if callIndex < len(s.deleteBatches) {
		batches = s.deleteBatches[callIndex]
	}

	return deleted, batches, nil
}

func (s *fakeProbeStore) VacuumAnalyzeProbeResults(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.vacuumCalls++
	return nil
}

type fakeProbeExecutor struct {
	result structs.ProbeResult
	err    error
}

func (e fakeProbeExecutor) Execute(_ context.Context, _ structs.ProbeConfig) (structs.ProbeResult, error) {
	return e.result, e.err
}

type slowProbeExecutor struct {
	delay  time.Duration
	result structs.ProbeResult
}

func (e slowProbeExecutor) Execute(_ context.Context, _ structs.ProbeConfig) (structs.ProbeResult, error) {
	time.Sleep(e.delay)
	return e.result, nil
}

func TestHTTPProbeExecutorExecuteSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	executor := &httpProbeExecutor{client: &http.Client{}}
	result, err := executor.Execute(context.Background(), structs.ProbeConfig{
		URL:            server.URL,
		Method:         http.MethodGet,
		ExpectedStatus: http.StatusOK,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !result.Success {
		t.Fatalf("Execute() success = false, want true")
	}
	if result.StatusCode == nil || *result.StatusCode != http.StatusOK {
		t.Fatalf("Execute() status code = %v, want 200", result.StatusCode)
	}
	if result.ResponseTimeMs == nil {
		t.Fatal("Execute() response time is nil, want a value")
	}
	if result.FailureType != "" {
		t.Fatalf("Execute() failure type = %q, want empty for success", result.FailureType)
	}
}

func TestHTTPProbeExecutorExecuteUnexpectedStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	executor := &httpProbeExecutor{client: &http.Client{}}
	result, err := executor.Execute(context.Background(), structs.ProbeConfig{
		URL:            server.URL,
		Method:         http.MethodGet,
		ExpectedStatus: http.StatusOK,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Success {
		t.Fatal("Execute() success = true, want false")
	}
	if result.StatusCode == nil || *result.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("Execute() status code = %v, want 503", result.StatusCode)
	}
	if result.ErrorMessage == "" {
		t.Fatal("Execute() error message is empty, want status mismatch detail")
	}
	if result.FailureType != structs.ProbeFailureTypeHTTPStatus {
		t.Fatalf("Execute() failure type = %q, want %q", result.FailureType, structs.ProbeFailureTypeHTTPStatus)
	}
}

func TestHTTPProbeExecutorExecuteClientStatusIsFunctional(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	executor := &httpProbeExecutor{client: &http.Client{}}
	result, err := executor.Execute(context.Background(), structs.ProbeConfig{
		URL:            server.URL,
		Method:         http.MethodGet,
		ExpectedStatus: http.StatusOK,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !result.Success {
		t.Fatal("Execute() success = false, want true for a functional 4xx response")
	}
	if result.StatusCode == nil || *result.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("Execute() status code = %v, want 429", result.StatusCode)
	}
	if result.FailureType != "" {
		t.Fatalf("Execute() failure type = %q, want empty for functional 4xx status", result.FailureType)
	}
	if result.ErrorMessage != "" {
		t.Fatalf("Execute() error message = %q, want empty for functional 4xx status", result.ErrorMessage)
	}
}

func TestHTTPProbeExecutorExecuteTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(150 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()

	executor := &httpProbeExecutor{client: &http.Client{}}
	result, err := executor.Execute(ctx, structs.ProbeConfig{
		URL:            server.URL,
		Method:         http.MethodGet,
		ExpectedStatus: http.StatusOK,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil because timeout is recorded as a probe failure", err)
	}
	if result.Success {
		t.Fatal("Execute() success = true, want false on timeout")
	}
	if result.StatusCode != nil {
		t.Fatalf("Execute() status code = %v, want nil on timeout", result.StatusCode)
	}
	if result.ErrorMessage == "" {
		t.Fatal("Execute() error message is empty, want timeout detail")
	}
	if result.FailureType != structs.ProbeFailureTypeTimeout {
		t.Fatalf("Execute() failure type = %q, want %q", result.FailureType, structs.ProbeFailureTypeTimeout)
	}
}

func TestHTTPProbeExecutorExecuteNetworkError(t *testing.T) {
	executor := &httpProbeExecutor{client: &http.Client{}}
	result, err := executor.Execute(context.Background(), structs.ProbeConfig{
		URL:            "http://127.0.0.1:1",
		Method:         http.MethodGet,
		ExpectedStatus: http.StatusOK,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil because network failure is recorded", err)
	}
	if result.Success {
		t.Fatal("Execute() success = true, want false")
	}
	if result.StatusCode != nil {
		t.Fatalf("Execute() status code = %v, want nil", result.StatusCode)
	}
	if result.ErrorMessage == "" {
		t.Fatal("Execute() error message is empty, want connection detail")
	}
	if result.FailureType != structs.ProbeFailureTypeConnect {
		t.Fatalf("Execute() failure type = %q, want %q", result.FailureType, structs.ProbeFailureTypeConnect)
	}
}

func TestHTTPProbeExecutorExecuteInvalidRequest(t *testing.T) {
	executor := &httpProbeExecutor{client: &http.Client{}}
	result, err := executor.Execute(context.Background(), structs.ProbeConfig{
		URL:            ":// bad url",
		Method:         http.MethodGet,
		ExpectedStatus: http.StatusOK,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Success {
		t.Fatal("Execute() success = true, want false")
	}
	if result.FailureType != structs.ProbeFailureTypeInvalidRequest {
		t.Fatalf("Execute() failure type = %q, want %q", result.FailureType, structs.ProbeFailureTypeInvalidRequest)
	}
}

func TestHTTPProbeExecutorExecuteTLSFailure(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	executor := &httpProbeExecutor{client: &http.Client{}}
	result, err := executor.Execute(context.Background(), structs.ProbeConfig{
		URL:            server.URL,
		Method:         http.MethodGet,
		ExpectedStatus: http.StatusOK,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Success {
		t.Fatal("Execute() success = true, want false")
	}
	if result.FailureType != structs.ProbeFailureTypeTLS {
		t.Fatalf("Execute() failure type = %q, want %q", result.FailureType, structs.ProbeFailureTypeTLS)
	}
}

func TestClassifyProbeFailure(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want structs.ProbeFailureType
	}{
		{
			name: "timeout",
			err:  context.DeadlineExceeded,
			want: structs.ProbeFailureTypeTimeout,
		},
		{
			name: "dns",
			err:  &net.DNSError{Err: "no such host", Name: "example.invalid"},
			want: structs.ProbeFailureTypeDNS,
		},
		{
			name: "connect",
			err:  &net.OpError{Op: "dial", Err: syscall.ECONNREFUSED},
			want: structs.ProbeFailureTypeConnect,
		},
		{
			name: "tls",
			err:  x509.UnknownAuthorityError{},
			want: structs.ProbeFailureTypeTLS,
		},
		{
			name: "unknown",
			err:  errors.New("mystery failure"),
			want: structs.ProbeFailureTypeUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyProbeFailure(tt.err); got != tt.want {
				t.Fatalf("classifyProbeFailure() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRunProbeSweepCompletesClaimedConfigs(t *testing.T) {
	store := &fakeProbeStore{
		claimed: [][]structs.ProbeConfig{
			{
				{ID: 1, ServiceID: 10, LeaseToken: "lease-a"},
				{ID: 2, ServiceID: 11, LeaseToken: "lease-b"},
			},
		},
	}

	err := runProbeSweep(context.Background(), store, fakeProbeExecutor{
		result: structs.ProbeResult{Region: probeRegionGlobal, Success: true},
	}, time.Now().UTC())
	if err != nil {
		t.Fatalf("runProbeSweep() error = %v", err)
	}

	if len(store.completed) != 2 {
		t.Fatalf("len(completed) = %d, want 2", len(store.completed))
	}
	if store.completed[0].configID != 1 || store.completed[1].configID != 2 {
		t.Fatalf("completed config IDs = %+v, want [1 2]", store.completed)
	}
}

func TestRunProbeSweepSetsCheckedAtAfterProbeExecution(t *testing.T) {
	store := &fakeProbeStore{
		claimed: [][]structs.ProbeConfig{
			{
				{ID: 1, ServiceID: 10, LeaseToken: "lease-a"},
			},
		},
	}

	start := time.Now().UTC()
	delay := 20 * time.Millisecond
	err := runProbeSweep(context.Background(), store, slowProbeExecutor{
		delay:  delay,
		result: structs.ProbeResult{Region: probeRegionGlobal, Success: true},
	}, start)
	if err != nil {
		t.Fatalf("runProbeSweep() error = %v", err)
	}

	if len(store.completed) != 1 {
		t.Fatalf("len(completed) = %d, want 1", len(store.completed))
	}
	if cutoff := start.Add(delay); store.completed[0].checkedAt.Before(cutoff) {
		t.Fatalf("checkedAt = %s, want after probe execution cutoff %s", store.completed[0].checkedAt, cutoff)
	}
}

func TestCleanupOldProbeResults(t *testing.T) {
	store := &fakeProbeStore{}
	now := time.Date(2026, time.January, 10, 12, 0, 0, 0, time.UTC)

	if err := cleanupOldProbeResults(context.Background(), store, now); err != nil {
		t.Fatalf("cleanupOldProbeResults() error = %v", err)
	}

	if len(store.cleanupCalls) != 1 {
		t.Fatalf("len(cleanupCalls) = %d, want 1", len(store.cleanupCalls))
	}
	call := store.cleanupCalls[0]
	if want := now.Add(-probeRawSuccessRetention); !call.successCutoff.Equal(want) {
		t.Fatalf("success cutoff = %s, want %s", call.successCutoff, want)
	}
	if want := now.Add(-probeRawFailureRetention); !call.failureCutoff.Equal(want) {
		t.Fatalf("failure cutoff = %s, want %s", call.failureCutoff, want)
	}
	if call.batchSize != probeCleanupBatchSize {
		t.Fatalf("batch size = %d, want %d", call.batchSize, probeCleanupBatchSize)
	}
	if call.maxBatches != probeCleanupMaxBatches {
		t.Fatalf("max batches = %d, want %d", call.maxBatches, probeCleanupMaxBatches)
	}
	if store.vacuumCalls != 0 {
		t.Fatalf("vacuum calls = %d, want 0", store.vacuumCalls)
	}
}

func TestCleanupOldProbeResultsVacuumsAfterLargePurge(t *testing.T) {
	store := &fakeProbeStore{
		deleteRows: []int64{probeCleanupVacuumThreshold},
	}
	now := time.Date(2026, time.January, 10, 12, 0, 0, 0, time.UTC)

	if err := cleanupOldProbeResults(context.Background(), store, now); err != nil {
		t.Fatalf("cleanupOldProbeResults() error = %v", err)
	}

	if store.vacuumCalls != 1 {
		t.Fatalf("vacuum calls = %d, want 1", store.vacuumCalls)
	}
}

func TestRunProbeWorkerReturnsOnCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	store := &fakeProbeStore{}
	err := runProbeWorker(ctx, store, fakeProbeExecutor{})
	if err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("runProbeWorker() error = %v, want nil or context.Canceled", err)
	}
	if len(store.cleanupCalls) != 0 {
		t.Fatalf("runProbeWorker() cleaned old probe results %d times, want 0", len(store.cleanupCalls))
	}
}
