package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type IssueStore interface {
	ListOpenIssues(ctx context.Context) ([]Issue, error)
	AssignIssue(ctx context.Context, number int, assignee string) error
}

type PRStore interface {
	GetPR(ctx context.Context, number int) (PullRequest, error)
	DiffPR(ctx context.Context, number int) (string, error)
	PostReview(ctx context.Context, number int, body string) error
}

type GHClient struct {
	Binary string
	Dir    string
}

func NewGHClient(dir string) GHClient {
	return GHClient{Binary: "gh", Dir: dir}
}

func (g GHClient) ListOpenIssues(ctx context.Context) ([]Issue, error) {
	out, err := g.run(ctx, "issue", "list", "--state", "open", "--limit", "100", "--json", "number,title,body,url,labels,assignees")
	if err != nil {
		return nil, err
	}

	var decoded []struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		Body   string `json:"body"`
		URL    string `json:"url"`
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
		Assignees []struct {
			Login string `json:"login"`
		} `json:"assignees"`
	}
	if err := json.Unmarshal(out, &decoded); err != nil {
		return nil, fmt.Errorf("decode gh issue list: %w", err)
	}

	issues := make([]Issue, 0, len(decoded))
	for _, item := range decoded {
		issue := Issue{
			Number: item.Number,
			Title:  item.Title,
			Body:   item.Body,
			URL:    item.URL,
		}
		for _, label := range item.Labels {
			issue.Labels = append(issue.Labels, label.Name)
		}
		for _, assignee := range item.Assignees {
			issue.Assignees = append(issue.Assignees, assignee.Login)
		}
		issues = append(issues, issue)
	}
	return issues, nil
}

func (g GHClient) AssignIssue(ctx context.Context, number int, assignee string) error {
	_, err := g.run(ctx, "issue", "edit", strconv.Itoa(number), "--add-assignee", assignee)
	return err
}

func (g GHClient) GetPR(ctx context.Context, number int) (PullRequest, error) {
	out, err := g.run(ctx, "pr", "view", strconv.Itoa(number), "--json", "number,title,body,url,author")
	if err != nil {
		return PullRequest{}, err
	}

	var decoded struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		Body   string `json:"body"`
		URL    string `json:"url"`
		Author struct {
			Login string `json:"login"`
		} `json:"author"`
	}
	if err := json.Unmarshal(out, &decoded); err != nil {
		return PullRequest{}, fmt.Errorf("decode gh pr view: %w", err)
	}

	return PullRequest{
		Number: decoded.Number,
		Title:  decoded.Title,
		Body:   decoded.Body,
		URL:    decoded.URL,
		Author: decoded.Author.Login,
	}, nil
}

func (g GHClient) DiffPR(ctx context.Context, number int) (string, error) {
	out, err := g.run(ctx, "pr", "diff", strconv.Itoa(number))
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func (g GHClient) PostReview(ctx context.Context, number int, body string) error {
	file, err := os.CreateTemp("", fmt.Sprintf("dispatch-pr-%d-review-*.md", number))
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())

	if _, err := file.WriteString(body); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}

	_, err = g.run(ctx, "pr", "review", strconv.Itoa(number), "--comment", "--body-file", file.Name())
	return err
}

func (g GHClient) run(ctx context.Context, args ...string) ([]byte, error) {
	binary := g.Binary
	if binary == "" {
		binary = "gh"
	}

	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = g.Dir
	cmd.Env = os.Environ()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return stdout.Bytes(), fmt.Errorf("gh %s: %w: %s", strings.Join(args, " "), err, msg)
		}
		return stdout.Bytes(), fmt.Errorf("gh %s: %w", strings.Join(args, " "), err)
	}
	return stdout.Bytes(), nil
}
