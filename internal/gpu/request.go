package gpu

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type RequestState string

const (
	RequestStatePending   RequestState = "Pending"
	RequestStateAdmitted  RequestState = "Admitted"
	RequestStateRunning   RequestState = "Running"
	RequestStateCompleted RequestState = "Completed"
	RequestStateCancelled RequestState = "Cancelled"
	RequestStateFailed    RequestState = "Failed"
)

type GPURequest struct {
	ID        string       `json:"id"`
	Tenant    string       `json:"tenant"`
	GPUType   string       `json:"gpuType"`
	Quantity  int          `json:"quantity"`
	State     RequestState `json:"state"`
	Cluster   string       `json:"cluster,omitempty"`
	Reason    string       `json:"reason,omitempty"`
	CreatedAt time.Time    `json:"createdAt"`
	UpdatedAt time.Time    `json:"updatedAt"`
}

type AdmissionResult struct {
	Admitted bool         `json:"admitted"`
	Cluster  string       `json:"cluster,omitempty"`
	Reason   string       `json:"reason,omitempty"`
	State    RequestState `json:"state"`
}

var (
	ErrRequestNotFound    = errors.New("request not found")
	ErrDuplicateRequest   = errors.New("request with this ID already exists")
	ErrRequestTerminal    = errors.New("request is in a terminal state")
	ErrRequestCancelled   = errors.New("request has been cancelled")
	ErrRequestNotPending  = errors.New("request is not in Pending state")
	ErrRequestNotAdmitted = errors.New("only Admitted requests can transition to Running")
)

func (m *Manager) SubmitRequest(ctx context.Context, req GPURequest) (*AdmissionResult, error) {
	if req.ID == "" {
		return nil, fmt.Errorf("request ID is required")
	}
	if req.Tenant == "" {
		return nil, fmt.Errorf("tenant is required")
	}
	if req.GPUType == "" {
		return nil, fmt.Errorf("GPU type is required")
	}
	if req.Quantity <= 0 {
		return nil, fmt.Errorf("quantity must be positive")
	}

	m.mu.Lock()
	if _, exists := m.reqs[req.ID]; exists {
		m.mu.Unlock()
		return nil, ErrDuplicateRequest
	}

	now := time.Now()
	req.State = RequestStatePending
	req.CreatedAt = now
	req.UpdatedAt = now
	m.reqs[req.ID] = &req
	m.mu.Unlock()

	m.logger.Info("gpu.SubmitRequest", "id", req.ID, "tenant", req.Tenant, "gpuType", req.GPUType)

	cluster, routeErr := m.BestClusterWithTimeout(ctx, req.GPUType, m.requestTimeout())

	m.mu.Lock()
	defer m.mu.Unlock()

	stored := m.reqs[req.ID]
	if stored.State != RequestStatePending {
		return &AdmissionResult{
			Admitted: false,
			Reason:   fmt.Sprintf("request moved to %s during routing", stored.State),
			State:    stored.State,
		}, nil
	}

	if routeErr != nil {
		reason := "placement pending"
		if errors.Is(routeErr, ErrGPUNoMatch) {
			reason = "no matching cluster"
		} else if !errors.Is(routeErr, ErrGPUPending) {
			return nil, fmt.Errorf("routing request: %w", routeErr)
		}

		stored.Reason = reason
		stored.UpdatedAt = time.Now()
		return &AdmissionResult{
			Admitted: false,
			Reason:   reason,
			State:    RequestStatePending,
		}, nil
	}

	stored.State = RequestStateAdmitted
	stored.Cluster = cluster
	stored.UpdatedAt = time.Now()

	return &AdmissionResult{
		Admitted: true,
		Cluster:  cluster,
		State:    RequestStateAdmitted,
	}, nil
}

func (m *Manager) GetRequestStatus(_ context.Context, requestID string) (*GPURequest, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	req, ok := m.reqs[requestID]
	if !ok {
		return nil, ErrRequestNotFound
	}

	copy := *req
	return &copy, nil
}

func (m *Manager) CancelRequest(_ context.Context, requestID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	req, ok := m.reqs[requestID]
	if !ok {
		return ErrRequestNotFound
	}

	if req.State == RequestStateCompleted || req.State == RequestStateCancelled {
		return ErrRequestTerminal
	}

	req.State = RequestStateCancelled
	req.UpdatedAt = time.Now()
	return nil
}

func (m *Manager) RetryRequest(ctx context.Context, requestID string) (*AdmissionResult, error) {
	m.mu.Lock()
	req, ok := m.reqs[requestID]
	if !ok {
		m.mu.Unlock()
		return nil, ErrRequestNotFound
	}
	if req.State != RequestStatePending {
		m.mu.Unlock()
		return nil, ErrRequestNotPending
	}
	gpuType := req.GPUType
	m.mu.Unlock()

	m.logger.Info("gpu.RetryRequest", "id", requestID, "gpuType", gpuType)

	cluster, routeErr := m.BestClusterWithTimeout(ctx, gpuType, m.requestTimeout())

	m.mu.Lock()
	defer m.mu.Unlock()

	stored := m.reqs[requestID]
	if stored.State != RequestStatePending {
		return &AdmissionResult{
			Admitted: false,
			Reason:   fmt.Sprintf("request moved to %s during routing", stored.State),
			State:    stored.State,
		}, nil
	}

	if routeErr != nil {
		reason := "placement pending"
		if errors.Is(routeErr, ErrGPUNoMatch) {
			reason = "no matching cluster"
		} else if !errors.Is(routeErr, ErrGPUPending) {
			return nil, fmt.Errorf("routing request: %w", routeErr)
		}
		stored.Reason = reason
		stored.UpdatedAt = time.Now()
		return &AdmissionResult{
			Admitted: false,
			Reason:   reason,
			State:    RequestStatePending,
		}, nil
	}

	stored.State = RequestStateAdmitted
	stored.Cluster = cluster
	stored.Reason = ""
	stored.UpdatedAt = time.Now()
	return &AdmissionResult{
		Admitted: true,
		Cluster:  cluster,
		State:    RequestStateAdmitted,
	}, nil
}

func (m *Manager) StartRequest(_ context.Context, requestID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	req, ok := m.reqs[requestID]
	if !ok {
		return ErrRequestNotFound
	}
	if req.State != RequestStateAdmitted {
		return ErrRequestNotAdmitted
	}

	req.State = RequestStateRunning
	req.UpdatedAt = time.Now()
	return nil
}

func (m *Manager) CompleteRequest(_ context.Context, requestID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	req, ok := m.reqs[requestID]
	if !ok {
		return ErrRequestNotFound
	}

	if req.State == RequestStateCancelled {
		return ErrRequestCancelled
	}
	if req.State != RequestStateRunning && req.State != RequestStateAdmitted {
		return fmt.Errorf("cannot complete request in %s state", req.State)
	}

	req.State = RequestStateCompleted
	req.UpdatedAt = time.Now()
	return nil
}
