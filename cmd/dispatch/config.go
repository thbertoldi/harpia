package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const dispatchConfigName = ".dispatch.toml"

type Config struct {
	Agents     map[string]Agent
	QuotaHints map[string]string
}

type Agent struct {
	Name         string
	Command      []string
	Complexities []string
	Authors      []string
	Provider     string
	QuotaHint    string
}

func LoadConfigFromRepo(start string) (Config, string, error) {
	root, err := findRepoRoot(start)
	if err != nil {
		return Config{}, "", err
	}

	cfg, err := LoadConfig(filepath.Join(root, dispatchConfigName))
	if err != nil {
		return Config{}, "", err
	}
	return cfg, root, nil
}

func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read %s: %w", path, err)
	}

	cfg, err := parseDispatchTOML(string(data))
	if err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := validateRoster(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func findRepoRoot(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, dispatchConfigName)); err == nil {
			return dir, nil
		}
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return "", fmt.Errorf("%s not found at repo root %s", dispatchConfigName, dir)
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("%s not found from %s", dispatchConfigName, start)
		}
		dir = parent
	}
}

func validateRoster(cfg Config) error {
	if len(cfg.Agents) == 0 {
		return fmt.Errorf("agent roster is empty")
	}

	names := make([]string, 0, len(cfg.Agents))
	for name := range cfg.Agents {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		agent := cfg.Agents[name]
		if strings.TrimSpace(agent.Name) == "" {
			return fmt.Errorf("agent %q has empty name", name)
		}
		if len(agent.Command) == 0 {
			return fmt.Errorf("agent %q has empty command", name)
		}
		if !commandHasPromptPlaceholder(agent.Command) {
			return fmt.Errorf("agent %q command must include {prompt_file}", name)
		}
		if len(agent.Complexities) == 0 {
			return fmt.Errorf("agent %q has no complexities", name)
		}
	}
	return nil
}

func parseDispatchTOML(input string) (Config, error) {
	cfg := Config{
		Agents:     map[string]Agent{},
		QuotaHints: map[string]string{},
	}

	var section string
	var agentName string

	lines := strings.Split(input, "\n")
	for i, raw := range lines {
		lineNo := i + 1
		line := strings.TrimSpace(stripTOMLComment(raw))
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "["), "]"))
			agentName = ""
			if strings.HasPrefix(section, "agents.") {
				agentName = strings.TrimSpace(strings.TrimPrefix(section, "agents."))
				if agentName == "" {
					return Config{}, fmt.Errorf("line %d: empty agent section", lineNo)
				}
				agent := cfg.Agents[agentName]
				agent.Name = agentName
				cfg.Agents[agentName] = agent
				continue
			}
			if section == "quota" {
				continue
			}
			return Config{}, fmt.Errorf("line %d: unsupported section %q", lineNo, section)
		}

		key, rawValue, ok := strings.Cut(line, "=")
		if !ok {
			return Config{}, fmt.Errorf("line %d: expected key = value", lineNo)
		}
		key = strings.TrimSpace(key)
		rawValue = strings.TrimSpace(rawValue)
		if key == "" {
			return Config{}, fmt.Errorf("line %d: empty key", lineNo)
		}

		switch {
		case strings.HasPrefix(section, "agents."):
			agent := cfg.Agents[agentName]
			if err := applyAgentConfig(&agent, key, rawValue, lineNo); err != nil {
				return Config{}, err
			}
			cfg.Agents[agentName] = agent
		case section == "quota":
			value, err := parseTOMLString(rawValue)
			if err != nil {
				return Config{}, fmt.Errorf("line %d: quota %q: %w", lineNo, key, err)
			}
			cfg.QuotaHints[key] = value
		default:
			return Config{}, fmt.Errorf("line %d: key %q outside a supported section", lineNo, key)
		}
	}

	for name, agent := range cfg.Agents {
		agent.Name = name
		agent.Complexities = normalizeList(agent.Complexities)
		agent.Authors = normalizeAuthors(agent.Authors)
		cfg.Agents[name] = agent
	}

	return cfg, nil
}

func applyAgentConfig(agent *Agent, key, rawValue string, lineNo int) error {
	switch key {
	case "command", "argv":
		if strings.HasPrefix(rawValue, "[") {
			values, err := parseTOMLStringArray(rawValue)
			if err != nil {
				return fmt.Errorf("line %d: command: %w", lineNo, err)
			}
			agent.Command = values
			return nil
		}

		value, err := parseTOMLString(rawValue)
		if err != nil {
			return fmt.Errorf("line %d: command: %w", lineNo, err)
		}
		command, err := splitCommand(value)
		if err != nil {
			return fmt.Errorf("line %d: command: %w", lineNo, err)
		}
		agent.Command = command
	case "complexities":
		values, err := parseTOMLStringArray(rawValue)
		if err != nil {
			return fmt.Errorf("line %d: complexities: %w", lineNo, err)
		}
		agent.Complexities = values
	case "authors":
		values, err := parseTOMLStringArray(rawValue)
		if err != nil {
			return fmt.Errorf("line %d: authors: %w", lineNo, err)
		}
		agent.Authors = values
	case "provider":
		value, err := parseTOMLString(rawValue)
		if err != nil {
			return fmt.Errorf("line %d: provider: %w", lineNo, err)
		}
		agent.Provider = value
	case "quota_hint":
		value, err := parseTOMLString(rawValue)
		if err != nil {
			return fmt.Errorf("line %d: quota_hint: %w", lineNo, err)
		}
		agent.QuotaHint = value
	default:
		return fmt.Errorf("line %d: unsupported agent key %q", lineNo, key)
	}
	return nil
}

func stripTOMLComment(line string) string {
	inQuote := rune(0)
	escaped := false
	for i, r := range line {
		if escaped {
			escaped = false
			continue
		}
		if inQuote == '"' && r == '\\' {
			escaped = true
			continue
		}
		if r == '"' || r == '\'' {
			if inQuote == 0 {
				inQuote = r
				continue
			}
			if inQuote == r {
				inQuote = 0
				continue
			}
		}
		if r == '#' && inQuote == 0 {
			return line[:i]
		}
	}
	return line
}

func parseTOMLString(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("empty string")
	}

	if strings.HasPrefix(raw, "'") && strings.HasSuffix(raw, "'") {
		return strings.TrimSuffix(strings.TrimPrefix(raw, "'"), "'"), nil
	}
	if strings.HasPrefix(raw, "\"") && strings.HasSuffix(raw, "\"") {
		value, err := strconv.Unquote(raw)
		if err != nil {
			return "", err
		}
		return value, nil
	}
	return "", fmt.Errorf("expected quoted string")
}

func parseTOMLStringArray(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "[") || !strings.HasSuffix(raw, "]") {
		return nil, fmt.Errorf("expected string array")
	}

	body := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(raw, "["), "]"))
	if body == "" {
		return nil, nil
	}

	var values []string
	for body != "" {
		body = strings.TrimLeft(body, " \t")
		if body == "" {
			break
		}

		quote := body[0]
		if quote != '"' && quote != '\'' {
			return nil, fmt.Errorf("expected quoted array item near %q", body)
		}

		end := -1
		escaped := false
		for i := 1; i < len(body); i++ {
			if escaped {
				escaped = false
				continue
			}
			if quote == '"' && body[i] == '\\' {
				escaped = true
				continue
			}
			if body[i] == quote {
				end = i
				break
			}
		}
		if end == -1 {
			return nil, fmt.Errorf("unterminated array item")
		}

		value, err := parseTOMLString(body[:end+1])
		if err != nil {
			return nil, err
		}
		values = append(values, value)

		body = strings.TrimLeft(body[end+1:], " \t")
		if body == "" {
			break
		}
		if body[0] != ',' {
			return nil, fmt.Errorf("expected comma near %q", body)
		}
		body = body[1:]
	}
	return values, nil
}

func splitCommand(command string) ([]string, error) {
	var args []string
	var current strings.Builder
	quote := rune(0)
	escaped := false

	flush := func() {
		if current.Len() > 0 {
			args = append(args, current.String())
			current.Reset()
		}
	}

	for _, r := range command {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}
		if quote != '\'' && r == '\\' {
			escaped = true
			continue
		}
		if r == '"' || r == '\'' {
			if quote == 0 {
				quote = r
				continue
			}
			if quote == r {
				quote = 0
				continue
			}
		}
		if quote == 0 && (r == ' ' || r == '\t' || r == '\n') {
			flush()
			continue
		}
		current.WriteRune(r)
	}
	if escaped {
		current.WriteRune('\\')
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated quote")
	}
	flush()
	if len(args) == 0 {
		return nil, fmt.Errorf("empty command")
	}
	return args, nil
}

func normalizeList(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		normalized := strings.ToLower(strings.TrimSpace(value))
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	return out
}

func normalizeAuthors(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		normalized := normalizeAuthor(value)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	return out
}

func commandHasPromptPlaceholder(command []string) bool {
	for _, arg := range command {
		if strings.Contains(arg, "{prompt_file}") {
			return true
		}
	}
	return false
}
