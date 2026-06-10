package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type AgentRunner interface {
	Invocation(agent Agent, promptFile string) ([]string, error)
	Run(ctx context.Context, agent Agent, promptFile string) (string, error)
}

type ExecAgentRunner struct {
	Dir string
}

func (r ExecAgentRunner) Invocation(agent Agent, promptFile string) ([]string, error) {
	return buildAgentCommand(agent, promptFile)
}

func (r ExecAgentRunner) Run(ctx context.Context, agent Agent, promptFile string) (string, error) {
	argv, err := buildAgentCommand(agent, promptFile)
	if err != nil {
		return "", err
	}
	if len(argv) == 0 {
		return "", fmt.Errorf("agent %q command is empty", agent.Name)
	}

	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = r.Dir
	cmd.Env = os.Environ()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return stdout.String(), fmt.Errorf("run %s: %w: %s", shellJoin(argv), err, msg)
		}
		return stdout.String(), fmt.Errorf("run %s: %w", shellJoin(argv), err)
	}
	return stdout.String(), nil
}

func buildAgentCommand(agent Agent, promptFile string) ([]string, error) {
	if len(agent.Command) == 0 {
		return nil, fmt.Errorf("agent %q command is empty", agent.Name)
	}
	if !commandHasPromptPlaceholder(agent.Command) {
		return nil, fmt.Errorf("agent %q command must include {prompt_file}", agent.Name)
	}

	argv := make([]string, len(agent.Command))
	for i, arg := range agent.Command {
		argv[i] = strings.ReplaceAll(arg, "{prompt_file}", promptFile)
	}
	return argv, nil
}

func shellJoin(argv []string) string {
	quoted := make([]string, 0, len(argv))
	for _, arg := range argv {
		quoted = append(quoted, shellQuote(arg))
	}
	return strings.Join(quoted, " ")
}

func shellQuote(arg string) string {
	if arg == "" {
		return "''"
	}
	if strings.IndexFunc(arg, func(r rune) bool {
		return !(r == '_' || r == '-' || r == '.' || r == '/' || r == ':' || r == '=' ||
			(r >= '0' && r <= '9') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= 'a' && r <= 'z'))
	}) == -1 {
		return arg
	}
	return "'" + strings.ReplaceAll(arg, "'", "'\\''") + "'"
}
