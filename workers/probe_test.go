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
		configID uint
		token    string
		result   structs.ProbeResult
	}
	deleteCutoffs []time.Time
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

func (s *fakeProbeStore) CompleteProbeLease(_ context.Context, configID uint, leaseToken string, result structs.ProbeResult, _ time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.completed = append(s.completed, struct {
		configID uint
		token    string
		result   structs.ProbeResult
	}{
		configID: configID,
		token:    leaseToken,
		result:   result,
	})
	return nil
}

func (s *fakeProbeStore) DeleteProbeResultsOlderThan(_ context.Context, cutoff time.Time) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deleteCutoffs = append(s.deleteCutoffs, cutoff)
	return 3, nil
}

type fakeProbeExecutor struct {
	result structs.ProbeResult
	err    error
}

func (e fakeProbeExecutor) Execute(_ context.Context, _ structs.ProbeConfig) (structs.ProbeResult, error) {
	return e.result, e.err
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

func TestCleanupOldProbeResults(t *testing.T) {
	store := &fakeProbeStore{}
	now := time.Date(2026, time.January, 10, 12, 0, 0, 0, time.UTC)

	if err := cleanupOldProbeResults(context.Background(), store, now); err != nil {
		t.Fatalf("cleanupOldProbeResults() error = %v", err)
	}

	if len(store.deleteCutoffs) != 1 {
		t.Fatalf("len(deleteCutoffs) = %d, want 1", len(store.deleteCutoffs))
	}
	if want := now.Add(-probeRawRetention); !store.deleteCutoffs[0].Equal(want) {
		t.Fatalf("cleanup cutoff = %s, want %s", store.deleteCutoffs[0], want)
	}
}

func TestRunProbeWorkerReturnsOnCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := runProbeWorker(ctx, &fakeProbeStore{}, fakeProbeExecutor{})
	if err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("runProbeWorker() error = %v, want nil or context.Canceled", err)
	}
}
