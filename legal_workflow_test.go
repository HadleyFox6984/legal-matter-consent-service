package legalconsent

import (
	"context"
	"encoding/json"
	"testing"
)

type recordingClient struct {
	writes []string
	checks []string
}

func (c *recordingClient) Check(_ context.Context, userID, category string) (json.RawMessage, error) {
	c.checks = append(c.checks, userID+":"+category)
	return json.RawMessage(`{"granted":true}`), nil
}

func (c *recordingClient) Grant(_ context.Context, userID, category string) (json.RawMessage, error) {
	c.writes = append(c.writes, "grant:"+userID+":"+category)
	return json.RawMessage(`{}`), nil
}

func (c *recordingClient) Revoke(_ context.Context, userID, category string) (json.RawMessage, error) {
	c.writes = append(c.writes, "revoke:"+userID+":"+category)
	return json.RawMessage(`{}`), nil
}

func TestWorkflowRoutesLegalDecision(t *testing.T) {
	tests := []struct {
		name     string
		action   MatterAction
		grant    bool
		category string
		write    string
	}{
		{"intake grant", MatterIntake, true, "matter-intake", "grant:user-42:matter-intake"},
		{"signed delivery revoke", SignedDocumentDelivery, false, "signed-document-delivery", "revoke:user-42:signed-document-delivery"},
		{"deadline grant", DeadlineFollowUp, true, "deadline-follow-up", "grant:user-42:deadline-follow-up"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &recordingClient{}
			workflow := NewWorkflow(client)
			result, err := workflow.Apply(context.Background(), DecisionInput{
				OperationID: "op-100", UserID: "user-42", MatterID: "matter-7", Action: tt.action, Grant: tt.grant,
			})
			if err != nil {
				t.Fatal(err)
			}
			if result.Category != tt.category || len(client.writes) != 1 || client.writes[0] != tt.write {
				t.Fatalf("result=%+v writes=%v", result, client.writes)
			}
			if len(client.checks) != 1 || client.checks[0] != "user-42:"+tt.category {
				t.Fatalf("checks=%v", client.checks)
			}
		})
	}
}

func TestWorkflowDeduplicatesOperation(t *testing.T) {
	client := &recordingClient{}
	workflow := NewWorkflow(client)
	input := DecisionInput{OperationID: "op-101", UserID: "user-42", MatterID: "matter-7", Action: MatterIntake, Grant: true}
	if _, err := workflow.Apply(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	result, err := workflow.Apply(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Duplicate || len(client.writes) != 1 {
		t.Fatalf("duplicate=%v writes=%v", result.Duplicate, client.writes)
	}
}
