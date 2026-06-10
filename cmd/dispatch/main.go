package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

var errNoEligibleIssues = errors.New("no eligible issues")

type Clock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time {
	return time.Now()
}

type Issue struct {
	Number    int
	Title     string
	Body      string
	URL       string
	Labels    []string
	Assignees []string
}

type PullRequest struct {
	Number int
	Title  string
	Body   string
	URL    string
	Author string
}

type App struct {
	Config   Config
	Issues   IssueStore
	PRs      PRStore
	Agents   AgentRunner
	Clock    Clock
	RepoRoot string
}

type nextOptions struct {
	Agent      string
	Complexity string
	Run        bool
}

type reviewOptions struct {
	Agent string
	PR    int
}

func main() {
	if err := runCLI(context.Background(), os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "dispatch:", err)
		os.Exit(1)
	}
}

func runCLI(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		printUsage(stderr)
		return fmt.Errorf("missing command")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	cfg, root, err := LoadConfigFromRepo(cwd)
	if err != nil {
		return err
	}

	app := App{
		Config:   cfg,
		Issues:   NewGHClient(root),
		PRs:      NewGHClient(root),
		Agents:   ExecAgentRunner{Dir: root},
		Clock:    systemClock{},
		RepoRoot: root,
	}

	switch args[0] {
	case "next":
		fs := flag.NewFlagSet("dispatch next", flag.ContinueOnError)
		fs.SetOutput(stderr)
		opts := nextOptions{}
		fs.StringVar(&opts.Agent, "agent", "", "agent name from .dispatch.toml")
		fs.StringVar(&opts.Complexity, "complexity", "", "optional complexity level")
		fs.BoolVar(&opts.Run, "run", false, "run the agent instead of only printing the invocation")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if opts.Agent == "" {
			return fmt.Errorf("next requires --agent")
		}
		return app.Next(ctx, opts, stdout)
	case "review":
		fs := flag.NewFlagSet("dispatch review", flag.ContinueOnError)
		fs.SetOutput(stderr)
		opts := reviewOptions{}
		fs.StringVar(&opts.Agent, "agent", "", "agent name from .dispatch.toml")
		fs.IntVar(&opts.PR, "pr", 0, "pull request number")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if opts.Agent == "" {
			return fmt.Errorf("review requires --agent")
		}
		if opts.PR <= 0 {
			return fmt.Errorf("review requires --pr <N>")
		}
		return app.Review(ctx, opts, stdout)
	case "quota":
		fs := flag.NewFlagSet("dispatch quota", flag.ContinueOnError)
		fs.SetOutput(stderr)
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		return app.Quota(stdout)
	case "-h", "--help", "help":
		printUsage(stdout)
		return nil
	default:
		printUsage(stderr)
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "usage:")
	fmt.Fprintln(w, "  dispatch next --agent <name> [--complexity <level>] [--run]")
	fmt.Fprintln(w, "  dispatch review --pr <N> --agent <name>")
	fmt.Fprintln(w, "  dispatch quota")
}

func (a App) Next(ctx context.Context, opts nextOptions, stdout io.Writer) error {
	issues, err := a.Issues.ListOpenIssues(ctx)
	if err != nil {
		return err
	}

	agent, issue, complexity, err := selectNextIssue(a.Config, opts.Agent, opts.Complexity, issues)
	if errors.Is(err, errNoEligibleIssues) {
		fmt.Fprintln(stdout, errNoEligibleIssues.Error())
		return nil
	}
	if err != nil {
		return err
	}

	if err := a.Issues.AssignIssue(ctx, issue.Number, "@me"); err != nil {
		return err
	}

	promptFile, err := writePromptFile("dispatch-issue-"+strconv.Itoa(issue.Number)+"-*.md", a.issuePrompt(agent, issue, complexity))
	if err != nil {
		return err
	}

	argv, err := a.Agents.Invocation(agent, promptFile)
	if err != nil {
		return err
	}

	fmt.Fprintf(stdout, "assigned issue #%d: %s\n", issue.Number, issue.Title)
	if issue.URL != "" {
		fmt.Fprintf(stdout, "url: %s\n", issue.URL)
	}
	fmt.Fprintf(stdout, "complexity: %s\n", complexity)
	fmt.Fprintf(stdout, "prompt: %s\n", promptFile)
	fmt.Fprintln(stdout, shellJoin(argv))

	if !opts.Run {
		return nil
	}

	output, err := a.Agents.Run(ctx, agent, promptFile)
	if output != "" {
		fmt.Fprint(stdout, output)
		if !strings.HasSuffix(output, "\n") {
			fmt.Fprintln(stdout)
		}
	}
	return err
}

func (a App) Review(ctx context.Context, opts reviewOptions, stdout io.Writer) error {
	agent, err := getAgent(a.Config, opts.Agent)
	if err != nil {
		return err
	}

	pr, err := a.PRs.GetPR(ctx, opts.PR)
	if err != nil {
		return err
	}
	if err := refuseSelfReview(agent, pr); err != nil {
		return err
	}

	diff, err := a.PRs.DiffPR(ctx, opts.PR)
	if err != nil {
		return err
	}

	promptFile, err := writePromptFile("dispatch-pr-"+strconv.Itoa(opts.PR)+"-review-*.md", a.reviewPrompt(agent, pr, diff))
	if err != nil {
		return err
	}

	output, err := a.Agents.Run(ctx, agent, promptFile)
	if err != nil {
		return err
	}
	if strings.TrimSpace(output) == "" {
		return fmt.Errorf("agent %q produced an empty review", agent.Name)
	}

	if err := a.PRs.PostReview(ctx, opts.PR, output); err != nil {
		return err
	}

	fmt.Fprintf(stdout, "posted review for PR #%d using %s\n", opts.PR, agent.Name)
	return nil
}

func (a App) Quota(stdout io.Writer) error {
	if err := validateRoster(a.Config); err != nil {
		return err
	}

	if len(a.Config.QuotaHints) == 0 {
		fmt.Fprintln(stdout, "no quota hints configured")
		return nil
	}

	fmt.Fprintln(stdout, "provider quota hints:")
	providers := make([]string, 0, len(a.Config.QuotaHints))
	for provider := range a.Config.QuotaHints {
		providers = append(providers, provider)
	}
	sort.Strings(providers)
	for _, provider := range providers {
		fmt.Fprintf(stdout, "- %s: %s\n", provider, a.Config.QuotaHints[provider])
	}

	agents := make([]string, 0, len(a.Config.Agents))
	for name := range a.Config.Agents {
		agents = append(agents, name)
	}
	sort.Strings(agents)
	for _, name := range agents {
		agent := a.Config.Agents[name]
		if strings.TrimSpace(agent.QuotaHint) == "" {
			continue
		}
		fmt.Fprintf(stdout, "- %s: %s\n", agent.Name, agent.QuotaHint)
	}
	return nil
}

func selectNextIssue(cfg Config, agentName, requestedComplexity string, issues []Issue) (Agent, Issue, string, error) {
	if err := validateRoster(cfg); err != nil {
		return Agent{}, Issue{}, "", err
	}

	agent, err := getAgent(cfg, agentName)
	if err != nil {
		return Agent{}, Issue{}, "", err
	}

	requestedComplexity = normalizeComplexity(requestedComplexity)
	if requestedComplexity != "" && !agentAcceptsComplexity(agent, requestedComplexity) {
		return Agent{}, Issue{}, "", fmt.Errorf("agent %q does not accept complexity %q", agent.Name, requestedComplexity)
	}

	for _, issue := range issues {
		if len(issue.Assignees) > 0 {
			continue
		}

		complexity, ok := issueComplexity(issue.Labels)
		if !ok {
			continue
		}
		if requestedComplexity != "" && complexity != requestedComplexity {
			continue
		}
		if !agentAcceptsComplexity(agent, complexity) {
			continue
		}
		return agent, issue, complexity, nil
	}

	return Agent{}, Issue{}, "", errNoEligibleIssues
}

func getAgent(cfg Config, name string) (Agent, error) {
	if err := validateRoster(cfg); err != nil {
		return Agent{}, err
	}
	agent, ok := cfg.Agents[name]
	if !ok {
		return Agent{}, fmt.Errorf("agent %q not found", name)
	}
	return agent, nil
}

func issueComplexity(labels []string) (string, bool) {
	for _, label := range labels {
		label = strings.TrimSpace(strings.ToLower(label))
		value, ok := strings.CutPrefix(label, "complexity:")
		if !ok {
			continue
		}
		value = normalizeComplexity(value)
		if value == "" {
			continue
		}
		return value, true
	}
	return "", false
}

func agentAcceptsComplexity(agent Agent, complexity string) bool {
	complexity = normalizeComplexity(complexity)
	for _, allowed := range agent.Complexities {
		if normalizeComplexity(allowed) == complexity {
			return true
		}
	}
	return false
}

func normalizeComplexity(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func refuseSelfReview(agent Agent, pr PullRequest) error {
	prAuthor := normalizeAuthor(pr.Author)
	if prAuthor == "" {
		return nil
	}

	authors := map[string]struct{}{
		normalizeAuthor(agent.Name): {},
	}
	for _, author := range agent.Authors {
		authors[normalizeAuthor(author)] = struct{}{}
	}

	if _, ok := authors[prAuthor]; ok {
		return fmt.Errorf("agent %q authored PR #%d as %q; refusing self-review", agent.Name, pr.Number, pr.Author)
	}
	return nil
}

func normalizeAuthor(value string) string {
	return strings.ToLower(strings.TrimPrefix(strings.TrimSpace(value), "@"))
}

func writePromptFile(pattern, content string) (string, error) {
	file, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", err
	}
	defer file.Close()

	if _, err := file.WriteString(content); err != nil {
		return "", err
	}
	return file.Name(), nil
}

func (a App) issuePrompt(agent Agent, issue Issue, complexity string) string {
	body := strings.TrimSpace(issue.Body)
	if body == "" {
		body = issue.Title
	}

	return fmt.Sprintf(`# Dispatch Issue #%d

Agent: %s
Repository: %s
Complexity: %s
Generated: %s

Title: %s
URL: %s

Task:
%s

Instructions:
- Work only in this repository and keep the change scoped to the issue.
- Follow existing project conventions and add focused validation.
- Do not push, open a PR, or make unrelated changes.
`, issue.Number, agent.Name, filepath.Clean(a.RepoRoot), complexity, a.now().Format(time.RFC3339), issue.Title, issue.URL, body)
}

func (a App) reviewPrompt(agent Agent, pr PullRequest, diff string) string {
	body := strings.TrimSpace(pr.Body)
	if body == "" {
		body = "(no PR body)"
	}

	return fmt.Sprintf(`# Review PR #%d

Agent: %s
Repository: %s
Generated: %s

Title: %s
Author: %s
URL: %s

PR Body:
%s

Review instructions:
- Review for correctness, regressions, security, missing tests, and maintainability.
- Lead with concrete findings that cite files and lines where possible.
- Keep the output suitable for a GitHub PR review comment.

Diff:
`+"```diff"+`
%s
`+"```"+`
`, pr.Number, agent.Name, filepath.Clean(a.RepoRoot), a.now().Format(time.RFC3339), pr.Title, pr.Author, pr.URL, body, diff)
}

func (a App) now() time.Time {
	if a.Clock == nil {
		return time.Now()
	}
	return a.Clock.Now()
}
