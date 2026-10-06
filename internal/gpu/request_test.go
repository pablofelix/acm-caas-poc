package gpu

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pablofelix/acm-caas-poc/internal/client"
)

func newRequestTestManager() *Manager {
	mgr := newTestManager()
	mgr.reqTimeout = 2 * time.Second
	return mgr
}

func simulatePlacementDecisions(ctx context.Context, mgr *Manager, cluster string) {
	for i := 0; i < 40; i++ {
		time.Sleep(5 * time.Millisecond)
		if ctx.Err() != nil {
			return
		}
		placements, _ := mgr.client.List(ctx, client.GVRPlacement, DefaultNamespace, "")
		for _, p := range placements.Items {
			name := p.GetName()
			pd := placementDecisionObj(name, cluster)
			_ = mgr.client.CreateIfNotExists(ctx, client.GVRPlacementDecision, DefaultNamespace, pd)
		}
	}
}

func seedRequest(mgr *Manager, req GPURequest) {
	now := time.Now()
	req.CreatedAt = now
	req.UpdatedAt = now
	mgr.reqs[req.ID] = &req
}

func validRequest() GPURequest {
	return GPURequest{
		ID:       "req-001",
		Tenant:   "team-alpha",
		GPUType:  "H100",
		Quantity: 1,
		State:    RequestStatePending,
	}
}

func TestSubmitRequestValidationEmptyID(t *testing.T) {
	mgr := newRequestTestManager()
	req := validRequest()
	req.ID = ""
	_, err := mgr.SubmitRequest(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for empty ID")
	}
}

func TestSubmitRequestValidationEmptyTenant(t *testing.T) {
	mgr := newRequestTestManager()
	req := validRequest()
	req.Tenant = ""
	_, err := mgr.SubmitRequest(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for empty tenant")
	}
}

func TestSubmitRequestValidationEmptyGPUType(t *testing.T) {
	mgr := newRequestTestManager()
	req := validRequest()
	req.GPUType = ""
	_, err := mgr.SubmitRequest(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for empty GPU type")
	}
}

func TestSubmitRequestValidationZeroQuantity(t *testing.T) {
	mgr := newRequestTestManager()
	req := validRequest()
	req.Quantity = 0
	_, err := mgr.SubmitRequest(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for zero quantity")
	}
}

func TestSubmitRequestDuplicateID(t *testing.T) {
	mgr := newRequestTestManager()
	seedRequest(mgr, validRequest())

	_, err := mgr.SubmitRequest(context.Background(), validRequest())
	if !errors.Is(err, ErrDuplicateRequest) {
		t.Fatalf("expected ErrDuplicateRequest, got: %v", err)
	}
}

func TestSubmitRequestAdmitted(t *testing.T) {
	mgr := newTestManager(gpuReadyCluster("gpu1", "H100"))
	mgr.reqTimeout = 2 * time.Second
	ctx := context.Background()

	go simulatePlacementDecisions(ctx, mgr, "gpu1")

	result, err := mgr.SubmitRequest(ctx, validRequest())
	if err != nil {
		t.Fatalf("SubmitRequest failed: %v", err)
	}
	if !result.Admitted {
		t.Errorf("expected Admitted=true, got reason: %s", result.Reason)
	}
	if result.Cluster != "gpu1" {
		t.Errorf("cluster = %q, want gpu1", result.Cluster)
	}
	if result.State != RequestStateAdmitted {
		t.Errorf("state = %q, want Admitted", result.State)
	}
}

func TestSubmitRequestPendingNoDecision(t *testing.T) {
	mgr := newTestManager(gpuReadyCluster("gpu1", "H100"))
	mgr.reqTimeout = 200 * time.Millisecond

	result, err := mgr.SubmitRequest(context.Background(), validRequest())
	if err != nil {
		t.Fatalf("SubmitRequest failed: %v", err)
	}
	if result.Admitted {
		t.Error("expected Admitted=false when no PlacementDecision exists")
	}
	if result.State != RequestStatePending {
		t.Errorf("state = %q, want Pending", result.State)
	}
	if result.Reason == "" {
		t.Error("expected a reason for pending admission")
	}
}

func TestSubmitRequestCancelledDuringRouting(t *testing.T) {
	mgr := newTestManager(gpuReadyCluster("gpu1", "H100"))
	mgr.reqTimeout = 500 * time.Millisecond
	ctx := context.Background()

	done := make(chan struct{})
	go func() {
		defer close(done)
		time.Sleep(50 * time.Millisecond)
		_ = mgr.CancelRequest(ctx, "req-001")
	}()

	result, err := mgr.SubmitRequest(ctx, validRequest())
	<-done
	if err != nil {
		t.Fatalf("SubmitRequest failed: %v", err)
	}
	if result.State != RequestStateCancelled {
		status, _ := mgr.GetRequestStatus(ctx, "req-001")
		if status.State == RequestStateAdmitted {
			t.Error("cancelled request must not transition to Admitted")
		}
	}
}

func TestGetRequestStatusFound(t *testing.T) {
	mgr := newRequestTestManager()
	seedRequest(mgr, validRequest())

	status, err := mgr.GetRequestStatus(context.Background(), "req-001")
	if err != nil {
		t.Fatalf("GetRequestStatus failed: %v", err)
	}
	if status.ID != "req-001" {
		t.Errorf("ID = %q, want req-001", status.ID)
	}
}

func TestGetRequestStatusNotFound(t *testing.T) {
	mgr := newRequestTestManager()
	_, err := mgr.GetRequestStatus(context.Background(), "nonexistent")
	if !errors.Is(err, ErrRequestNotFound) {
		t.Fatalf("expected ErrRequestNotFound, got: %v", err)
	}
}

func TestCancelRequestSuccess(t *testing.T) {
	mgr := newRequestTestManager()
	seedRequest(mgr, validRequest())

	err := mgr.CancelRequest(context.Background(), "req-001")
	if err != nil {
		t.Fatalf("CancelRequest failed: %v", err)
	}

	status, _ := mgr.GetRequestStatus(context.Background(), "req-001")
	if status.State != RequestStateCancelled {
		t.Errorf("state = %q, want Cancelled", status.State)
	}
}

func TestCancelRequestAlreadyCancelled(t *testing.T) {
	mgr := newRequestTestManager()
	req := validRequest()
	req.State = RequestStateCancelled
	seedRequest(mgr, req)

	err := mgr.CancelRequest(context.Background(), "req-001")
	if !errors.Is(err, ErrRequestTerminal) {
		t.Fatalf("expected ErrRequestTerminal, got: %v", err)
	}
}

func TestCancelRequestCompleted(t *testing.T) {
	mgr := newRequestTestManager()
	req := validRequest()
	req.State = RequestStateCompleted
	seedRequest(mgr, req)

	err := mgr.CancelRequest(context.Background(), "req-001")
	if !errors.Is(err, ErrRequestTerminal) {
		t.Fatalf("expected ErrRequestTerminal, got: %v", err)
	}
}

func TestCancelRequestNotFound(t *testing.T) {
	mgr := newRequestTestManager()
	err := mgr.CancelRequest(context.Background(), "nonexistent")
	if !errors.Is(err, ErrRequestNotFound) {
		t.Fatalf("expected ErrRequestNotFound, got: %v", err)
	}
}

func TestStartRequestSuccess(t *testing.T) {
	mgr := newRequestTestManager()
	req := validRequest()
	req.State = RequestStateAdmitted
	seedRequest(mgr, req)

	err := mgr.StartRequest(context.Background(), "req-001")
	if err != nil {
		t.Fatalf("StartRequest failed: %v", err)
	}

	status, _ := mgr.GetRequestStatus(context.Background(), "req-001")
	if status.State != RequestStateRunning {
		t.Errorf("state = %q, want Running", status.State)
	}
}

func TestStartRequestNotAdmitted(t *testing.T) {
	mgr := newRequestTestManager()
	seedRequest(mgr, validRequest())

	err := mgr.StartRequest(context.Background(), "req-001")
	if !errors.Is(err, ErrRequestNotAdmitted) {
		t.Fatalf("expected ErrRequestNotAdmitted, got: %v", err)
	}
}

func TestStartRequestNotFound(t *testing.T) {
	mgr := newRequestTestManager()
	err := mgr.StartRequest(context.Background(), "nonexistent")
	if !errors.Is(err, ErrRequestNotFound) {
		t.Fatalf("expected ErrRequestNotFound, got: %v", err)
	}
}

func TestCompleteRequestFromRunning(t *testing.T) {
	mgr := newRequestTestManager()
	req := validRequest()
	req.State = RequestStateRunning
	seedRequest(mgr, req)

	err := mgr.CompleteRequest(context.Background(), "req-001")
	if err != nil {
		t.Fatalf("CompleteRequest failed: %v", err)
	}

	status, _ := mgr.GetRequestStatus(context.Background(), "req-001")
	if status.State != RequestStateCompleted {
		t.Errorf("state = %q, want Completed", status.State)
	}
}

func TestCompleteRequestFromAdmitted(t *testing.T) {
	mgr := newRequestTestManager()
	req := validRequest()
	req.State = RequestStateAdmitted
	seedRequest(mgr, req)

	err := mgr.CompleteRequest(context.Background(), "req-001")
	if err != nil {
		t.Fatalf("CompleteRequest failed: %v", err)
	}

	status, _ := mgr.GetRequestStatus(context.Background(), "req-001")
	if status.State != RequestStateCompleted {
		t.Errorf("state = %q, want Completed", status.State)
	}
}

func TestCompleteRequestFromPendingRejected(t *testing.T) {
	mgr := newRequestTestManager()
	seedRequest(mgr, validRequest())

	err := mgr.CompleteRequest(context.Background(), "req-001")
	if err == nil {
		t.Fatal("expected error when completing from Pending")
	}
}

func TestCompleteRequestCancelled(t *testing.T) {
	mgr := newRequestTestManager()
	req := validRequest()
	req.State = RequestStateCancelled
	seedRequest(mgr, req)

	err := mgr.CompleteRequest(context.Background(), "req-001")
	if !errors.Is(err, ErrRequestCancelled) {
		t.Fatalf("expected ErrRequestCancelled, got: %v", err)
	}
}

func TestCompleteRequestNotFound(t *testing.T) {
	mgr := newRequestTestManager()
	err := mgr.CompleteRequest(context.Background(), "nonexistent")
	if !errors.Is(err, ErrRequestNotFound) {
		t.Fatalf("expected ErrRequestNotFound, got: %v", err)
	}
}

func TestRetryRequestSuccess(t *testing.T) {
	mgr := newTestManager(gpuReadyCluster("gpu1", "H100"))
	mgr.reqTimeout = 2 * time.Second
	ctx := context.Background()

	seedRequest(mgr, validRequest())

	go simulatePlacementDecisions(ctx, mgr, "gpu1")

	result, err := mgr.RetryRequest(ctx, "req-001")
	if err != nil {
		t.Fatalf("RetryRequest failed: %v", err)
	}
	if !result.Admitted {
		t.Errorf("expected Admitted=true, got reason: %s", result.Reason)
	}
	if result.Cluster != "gpu1" {
		t.Errorf("cluster = %q, want gpu1", result.Cluster)
	}
}

func TestRetryRequestNotPending(t *testing.T) {
	mgr := newRequestTestManager()
	req := validRequest()
	req.State = RequestStateAdmitted
	seedRequest(mgr, req)

	_, err := mgr.RetryRequest(context.Background(), "req-001")
	if !errors.Is(err, ErrRequestNotPending) {
		t.Fatalf("expected ErrRequestNotPending, got: %v", err)
	}
}

func TestRetryRequestCompletedNotOverwritten(t *testing.T) {
	mgr := newTestManager(gpuReadyCluster("gpu1", "H100"))
	mgr.reqTimeout = 500 * time.Millisecond
	ctx := context.Background()

	seedRequest(mgr, validRequest())

	done := make(chan struct{})
	go func() {
		defer close(done)
		time.Sleep(50 * time.Millisecond)
		mgr.mu.Lock()
		stored := mgr.reqs["req-001"]
		stored.State = RequestStateAdmitted
		stored.Cluster = "gpu1"
		mgr.mu.Unlock()
		_ = mgr.StartRequest(ctx, "req-001")
		_ = mgr.CompleteRequest(ctx, "req-001")
	}()

	go simulatePlacementDecisions(ctx, mgr, "gpu1")

	result, err := mgr.RetryRequest(ctx, "req-001")
	<-done
	if err != nil {
		t.Fatalf("RetryRequest failed: %v", err)
	}
	if result.Admitted {
		t.Error("late routing must not admit a completed request")
	}

	status, _ := mgr.GetRequestStatus(ctx, "req-001")
	if status.State != RequestStateCompleted {
		t.Errorf("state = %q, want Completed", status.State)
	}
}

func TestRetryRequestNotFound(t *testing.T) {
	mgr := newRequestTestManager()
	_, err := mgr.RetryRequest(context.Background(), "nonexistent")
	if !errors.Is(err, ErrRequestNotFound) {
		t.Fatalf("expected ErrRequestNotFound, got: %v", err)
	}
}

func TestGetRequestStatusReturnsCopy(t *testing.T) {
	mgr := newRequestTestManager()
	seedRequest(mgr, validRequest())

	status, _ := mgr.GetRequestStatus(context.Background(), "req-001")
	status.State = RequestStateFailed

	original, _ := mgr.GetRequestStatus(context.Background(), "req-001")
	if original.State == RequestStateFailed {
		t.Error("modifying returned copy should not affect stored request")
	}
}

func TestSubmitRequestAdmittedStatusPersisted(t *testing.T) {
	mgr := newTestManager(gpuReadyCluster("gpu1", "H100"))
	mgr.reqTimeout = 2 * time.Second
	ctx := context.Background()

	go simulatePlacementDecisions(ctx, mgr, "gpu1")

	_, err := mgr.SubmitRequest(ctx, validRequest())
	if err != nil {
		t.Fatalf("SubmitRequest failed: %v", err)
	}

	status, _ := mgr.GetRequestStatus(ctx, "req-001")
	if status.State != RequestStateAdmitted {
		t.Errorf("state = %q, want Admitted", status.State)
	}
	if status.Cluster != "gpu1" {
		t.Errorf("cluster = %q, want gpu1", status.Cluster)
	}
}

func TestFullLifecycleAdmitStartComplete(t *testing.T) {
	mgr := newTestManager(gpuReadyCluster("gpu1", "H100"))
	mgr.reqTimeout = 2 * time.Second
	ctx := context.Background()

	go simulatePlacementDecisions(ctx, mgr, "gpu1")

	result, err := mgr.SubmitRequest(ctx, validRequest())
	if err != nil {
		t.Fatalf("SubmitRequest failed: %v", err)
	}
	if result.State != RequestStateAdmitted {
		t.Fatalf("state = %q, want Admitted", result.State)
	}

	if err := mgr.StartRequest(ctx, "req-001"); err != nil {
		t.Fatalf("StartRequest failed: %v", err)
	}

	if err := mgr.CompleteRequest(ctx, "req-001"); err != nil {
		t.Fatalf("CompleteRequest failed: %v", err)
	}

	status, _ := mgr.GetRequestStatus(ctx, "req-001")
	if status.State != RequestStateCompleted {
		t.Errorf("state = %q, want Completed", status.State)
	}
}
