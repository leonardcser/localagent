package agent

import (
	"strings"
	"testing"

	"localagent/pkg/prompts"
)

func TestBuildSystemPromptExcludesHeartbeatByDefault(t *testing.T) {
	cb := NewContextBuilder(t.TempDir())

	prompt := cb.BuildSystemPrompt(false)
	if strings.Contains(prompt, prompts.HeartbeatSystem) {
		t.Fatal("expected non-heartbeat prompt to exclude heartbeat instructions")
	}
}

func TestBuildSystemPromptIncludesHeartbeatForHeartbeatSessions(t *testing.T) {
	cb := NewContextBuilder(t.TempDir())

	prompt := cb.BuildSystemPrompt(true)
	if !strings.Contains(prompt, prompts.HeartbeatSystem) {
		t.Fatal("expected heartbeat prompt to include heartbeat instructions")
	}
}
