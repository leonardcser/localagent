package tools

import (
	"context"
	"testing"

	"localagent/pkg/cron"
)

type stubJobExecutor struct {
	response          string
	err               error
	messageToolCalled bool
}

func (s *stubJobExecutor) ProcessDirectWithChannel(_ context.Context, _, _, _, _ string) (string, error) {
	return s.response, s.err
}

func (s *stubJobExecutor) WasMessageToolCalled() bool {
	return s.messageToolCalled
}

func TestCronExecuteJobErrorsOnEmptyFallbackResponse(t *testing.T) {
	tool := NewCronTool(nil, &stubJobExecutor{
		response:          emptyAgentResponse,
		messageToolCalled: false,
	}, nil)

	job := &cron.CronJob{
		ID: "job-1",
		Payload: cron.CronPayload{
			Kind:    "agentTurn",
			Message: "send something",
		},
		Delivery: &cron.CronDelivery{
			Mode:    "announce",
			Channel: "web",
			To:      "default",
		},
	}

	result, err := tool.ExecuteJob(context.Background(), job)
	if err == nil {
		t.Fatal("expected error for empty fallback response")
	}
	if result != "" {
		t.Fatalf("expected empty result, got %q", result)
	}
}

func TestCronExecuteJobSucceedsWhenMessageToolWasUsed(t *testing.T) {
	tool := NewCronTool(nil, &stubJobExecutor{
		response:          emptyAgentResponse,
		messageToolCalled: true,
	}, nil)

	job := &cron.CronJob{
		ID: "job-2",
		Payload: cron.CronPayload{
			Kind:    "agentTurn",
			Message: "send something",
		},
		Delivery: &cron.CronDelivery{
			Mode:    "announce",
			Channel: "web",
			To:      "default",
		},
	}

	result, err := tool.ExecuteJob(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "ok" {
		t.Fatalf("expected ok result, got %q", result)
	}
}
