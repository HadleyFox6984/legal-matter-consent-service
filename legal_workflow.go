package legalconsent

import (
	"context"
	"encoding/json"
	"sync"
)

type MatterAction string

const (
	MatterIntake           MatterAction = "matter_intake"
	SignedDocumentDelivery MatterAction = "signed_document_delivery"
	DeadlineFollowUp       MatterAction = "deadline_follow_up"
)

var consentCategory = map[MatterAction]string{
	MatterIntake:           "matter-intake",
	SignedDocumentDelivery: "signed-document-delivery",
	DeadlineFollowUp:       "deadline-follow-up",
}

type DecisionInput struct {
	OperationID string       `json:"operation_id"`
	UserID      string       `json:"user_id"`
	MatterID    string       `json:"matter_id"`
	Action      MatterAction `json:"action"`
	Grant       bool         `json:"grant"`
}

type DecisionResult struct {
	OperationID string          `json:"operation_id"`
	UserID      string          `json:"user_id"`
	MatterID    string          `json:"matter_id"`
	Category    string          `json:"category"`
	State       json.RawMessage `json:"state"`
	Duplicate   bool            `json:"duplicate"`
}

type Workflow struct {
	client ConsentClient
	mu     sync.Mutex
	seen   map[string]DecisionResult
}

type InputError struct {
	Message string
}

func (e *InputError) Error() string { return e.Message }

func NewWorkflow(client ConsentClient) *Workflow {
	return &Workflow{client: client, seen: make(map[string]DecisionResult)}
}

func (w *Workflow) Apply(ctx context.Context, in DecisionInput) (DecisionResult, error) {
	category, ok := consentCategory[in.Action]
	if !ok || in.OperationID == "" || in.UserID == "" || in.MatterID == "" {
		return DecisionResult{}, &InputError{Message: "operation_id, user_id, matter_id, and a known action are required"}
	}

	w.mu.Lock()
	if prior, exists := w.seen[in.OperationID]; exists {
		prior.Duplicate = true
		w.mu.Unlock()
		return prior, nil
	}
	w.mu.Unlock()

	var err error
	if in.Grant {
		_, err = w.client.Grant(ctx, in.UserID, category)
	} else {
		_, err = w.client.Revoke(ctx, in.UserID, category)
	}
	if err != nil {
		return DecisionResult{}, err
	}
	state, err := w.client.Check(ctx, in.UserID, category)
	if err != nil {
		return DecisionResult{}, err
	}

	result := DecisionResult{OperationID: in.OperationID, UserID: in.UserID, MatterID: in.MatterID, Category: category, State: state}
	w.mu.Lock()
	w.seen[in.OperationID] = result
	w.mu.Unlock()
	return result, nil
}
