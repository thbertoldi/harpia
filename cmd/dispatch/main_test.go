package main

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestSelectNextIssue(t *testing.T) {
	baseConfig := Config{Agents: map[string]Agent{
		"codex": {
			Name:         "codex",
			Command:      []string{"codex", "apply", "--task-file", "{prompt_file}"},
			Complexities: []string{"routine", "substantive"},
		},
		"small": {
			Name:         "small",
			Command:      []string{"small", "--prompt", "{prompt_file}"},
			Complexities: []string{"trivial", "routine"},
		},
	}}

	tests := []struct {
		name                string
		cfg                 Config
		agent               string
		requestedComplexity string
		issues              []Issue
		wantNumber          int
		wantErr             string
	}{
		{
			name:  "picks first open unassigned issue accepted by agent",
			cfg:   baseConfig,
			agent: "codex",
			issues: []Issue{
				{Number: 1, Title: "missing label"},
				{Number: 2, Title: "assigned", Labels: []string{"complexity:routine"}, Assignees: []string{"alice"}},
				{Number: 3, Title: "match", Labels: []string{"bug", "complexity:routine"}},
			},
			wantNumber: 3,
		},
		{
			name:                "complexity flag outside agent roster refuses",
			cfg:                 baseConfig,
			agent:               "small",
			requestedComplexity: "substantive",
			issues:              []Issue{{Number: 4, Labels: []string{"complexity:substantive"}}},
			wantErr:             `agent "small" does not accept complexity "substantive"`,
		},
		{
			name:    "complexity mismatch has no eligible issue",
			cfg:     baseConfig,
			agent:   "small",
			issues:  []Issue{{Number: 5, Labels: []string{"complexity:substantive"}}},
			wantErr: errNoEligibleIssues.Error(),
		},
		{
			name:    "missing complexity label has no eligible issue",
			cfg:     baseConfig,
			agent:   "codex",
			issues:  []Issue{{Number: 6, Labels: []string{"bug"}}},
			wantErr: errNoEligibleIssues.Error(),
		},
		{
			name:    "empty roster refuses",
			cfg:     Config{Agents: map[string]Agent{}},
			agent:   "codex",
			issues:  []Issue{{Number: 7, Labels: []string{"complexity:routine"}}},
			wantErr: "agent roster is empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, issue, _, err := selectNextIssue(tt.cfg, tt.agent, tt.requestedComplexity, tt.issues)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error %q", tt.wantErr)
				}
				if errors.Is(err, errNoEligibleIssues) && tt.wantErr == errNoEligibleIssues.Error() {
					return
				}
				if got := err.Error(); !strings.Contains(got, tt.wantErr) {
					t.Fatalf("error = %q, want containing %q", got, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("selectNextIssue returned error: %v", err)
			}
			if issue.Number != tt.wantNumber {
				t.Fatalf("issue.Number = %d, want %d", issue.Number, tt.wantNumber)
			}
		})
	}
}

func TestRefuseSelfReview(t *testing.T) {
	tests := []struct {
		name    string
		agent   Agent
		pr      PullRequest
		wantErr bool
	}{
		{
			name:    "agent name matches PR author",
			agent:   Agent{Name: "codex"},
			pr:      PullRequest{Number: 10, Author: "codex"},
			wantErr: true,
		},
		{
			name:    "configured author alias matches PR author",
			agent:   Agent{Name: "opencode-qwen", Authors: []string{"qwen-bot"}},
			pr:      PullRequest{Number: 11, Author: "qwen-bot"},
			wantErr: true,
		},
		{
			name:    "different author may review",
			agent:   Agent{Name: "codex", Authors: []string{"codex"}},
			pr:      PullRequest{Number: 12, Author: "opencode-qwen"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := refuseSelfReview(tt.agent, tt.pr)
			if tt.wantErr && err == nil {
				t.Fatal("expected refusal")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected review to be allowed, got %v", err)
			}
		})
	}
}

func TestReviewRefusesBeforeRunningAgent(t *testing.T) {
	store := &fakePRStore{pr: PullRequest{Number: 72, Author: "codex"}}
	runner := &fakeAgentRunner{}
	app := App{
		Config: Config{Agents: map[string]Agent{
			"codex": {
				Name:         "codex",
				Command:      []string{"codex", "apply", "--task-file", "{prompt_file}"},
				Complexities: []string{"routine"},
			},
		}},
		PRs:    store,
		Agents: runner,
	}

	err := app.Review(context.Background(), reviewOptions{Agent: "codex", PR: 72}, io.Discard)
	if err == nil {
		t.Fatal("expected self-review refusal")
	}
	if store.diffCalled {
		t.Fatal("diff should not be fetched for self-review")
	}
	if runner.runCalled {
		t.Fatal("agent should not run for self-review")
	}
}

type fakePRStore struct {
	pr         PullRequest
	diffCalled bool
	postCalled bool
}

func (f *fakePRStore) GetPR(context.Context, int) (PullRequest, error) {
	return f.pr, nil
}

func (f *fakePRStore) DiffPR(context.Context, int) (string, error) {
	f.diffCalled = true
	return "", nil
}

func (f *fakePRStore) PostReview(context.Context, int, string) error {
	f.postCalled = true
	return nil
}

type fakeAgentRunner struct {
	runCalled bool
}

func (f *fakeAgentRunner) Invocation(agent Agent, promptFile string) ([]string, error) {
	return buildAgentCommand(agent, promptFile)
}

func (f *fakeAgentRunner) Run(context.Context, Agent, string) (string, error) {
	f.runCalled = true
	return "review", nil
}
