#!/usr/bin/env bun

import { existsSync, mkdirSync, readFileSync, readdirSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

type JsonObject = Record<string, any>;

export type CommandResult = {
  exitCode: number;
  stdout: string;
  stderr: string;
};

export type CommandRunner = (command: string[]) => CommandResult;

export type ValidationInput = {
  config: JsonObject;
  mise: JsonObject;
  openspec: JsonObject;
  packageJson: JsonObject;
  packageLock: JsonObject;
  gitignore: string;
  agents: Map<string, JsonObject>;
  agentDocuments: Map<string, string>;
  adapters: Set<string>;
};

export const REQUIRED_AGENTS = new Map([
  ["orchestrator", "primary"],
  ["planner", "primary"],
  ["adjudicator", "subagent"],
  ["explorer", "subagent"],
  ["implementer", "subagent"],
  ["reviewer-fast", "subagent"],
  ["reviewer-senior", "subagent"],
  ["scut", "subagent"],
  ["sol", "subagent"],
  ["terra", "subagent"],
  ["verifier", "subagent"],
]);

export const APPROVED_SKILL_RULES = [
  "*",
  "harpia",
  "openspec-*",
  "chat-ui",
  "design-accessibility-auditor",
  "grpc-service-development",
  "helm-chart-scaffolding",
  "kubernetes-deployment",
  "monitoring-observability",
  "postgresql-database-engineering",
  "python-backend",
  "shadcn-svelte",
  "svelte-code-writer",
  "svelte-core-bestpractices",
  "tailwind-css-patterns",
  "temporal-developer",
];

export const ADAPTER_PATHS = [
  ".opencode/commands/opsx-apply.md",
  ".opencode/commands/opsx-archive.md",
  ".opencode/commands/opsx-explore.md",
  ".opencode/commands/opsx-propose.md",
  ".opencode/commands/opsx-sync.md",
  ".opencode/skills/openspec-apply-change/SKILL.md",
  ".opencode/skills/openspec-archive-change/SKILL.md",
  ".opencode/skills/openspec-explore/SKILL.md",
  ".opencode/skills/openspec-propose/SKILL.md",
  ".opencode/skills/openspec-sync-specs/SKILL.md",
];

const SAFE_GIT_RULES = new Set([
  "git status*",
  "git diff*",
  "git log*",
  "git show*",
  "git branch --show-current",
  "git rev-parse*",
  "git ls-files*",
]);

const ROOT_BASH_RULES = new Map([
  ["*", "ask"],
  ["git status*", "allow"],
  ["git diff*", "allow"],
  ["git log*", "allow"],
  ["git show*", "allow"],
  ["git branch --show-current", "allow"],
  ["git rev-parse*", "allow"],
  ["git ls-files*", "allow"],
  ["cd frontend && bun run lint", "allow"],
  ["cd frontend && bun run check", "allow"],
  ["cd frontend && bunx vitest run *", "allow"],
  ["cd agent-runtime && ruff check src/", "allow"],
  ["cd agent-runtime && uv run pytest", "allow"],
  ["cd control-plane && go test ./...", "allow"],
  ["cd proto && buf lint", "allow"],
  ["mise run buf-generate", "allow"],
  ["mise run helm-lint", "allow"],
  ["openspec validate --all --strict --no-interactive", "allow"],
  ["mise run opencode-validate", "allow"],
]);

const VERIFIER_BASH_RULES = new Map([
  ["*", "deny"],
  ["cd frontend && bun run lint", "allow"],
  ["cd frontend && bun run check", "allow"],
  ["cd frontend && bunx vitest run", "allow"],
  ["cd agent-runtime && ruff check src/", "allow"],
  ["cd agent-runtime && uv run pytest", "allow"],
  ["cd control-plane && go test ./...", "allow"],
  ["cd proto && buf lint", "allow"],
  ["mise run helm-lint", "allow"],
  ["openspec validate --all --strict --no-interactive", "allow"],
  ["mise run opencode-validate", "allow"],
]);

const READ_ONLY_AGENTS = new Set([
  "adjudicator",
  "explorer",
  "reviewer-fast",
  "reviewer-senior",
  "sol",
  "verifier",
]);

const WRITER_AGENTS = new Set(["implementer", "terra", "scut"]);
const FORBIDDEN_LOADED_AGENTS = new Set([
  "build",
  "plan",
  "explore",
  "general",
  "hard-problem",
]);
const FORBIDDEN_ALIASES = [
  /gpt-5\.5/i,
  /deepseek-reasoner/i,
  /gemini-3\.1-pro-preview/i,
  /@hard-problem\b/i,
];

function object(value: any): JsonObject {
  return value && typeof value === "object" && !Array.isArray(value)
    ? value
    : {};
}

function mapEquals(actual: JsonObject, expected: Map<string, string>): boolean {
  const entries = Object.entries(object(actual));
  const expectedEntries = [...expected.entries()];
  return (
    entries.length === expected.size &&
    entries.every(
      ([key, value], index) =>
        expectedEntries[index][0] === key &&
        expectedEntries[index][1] === value,
    )
  );
}

function pinnedPackage(spec: string): boolean {
  if (spec.includes("@latest")) return false;
  if (spec.startsWith("@")) return /^@[^/]+\/[^@]+@\d/.test(spec);
  return /^[^@]+@\d/.test(spec);
}

export function parseAgentDocument(
  document: string,
  path = "agent",
): {
  frontmatter: JsonObject;
  body: string;
} {
  const match = document.match(/^---\r?\n([\s\S]*?)\r?\n---\r?\n([\s\S]*)$/);
  if (!match) throw new Error(`${path}: missing YAML frontmatter`);
  const parsed = Bun.YAML.parse(match[1]);
  if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
    throw new Error(`${path}: frontmatter must be an object`);
  }
  return { frontmatter: parsed as JsonObject, body: match[2] };
}

function readJson(path: string): JsonObject {
  return JSON.parse(readFileSync(path, "utf8"));
}

export function loadProject(root: string): ValidationInput {
  const agentDirectory = join(root, ".opencode", "agent");
  const agents = new Map<string, JsonObject>();
  const agentDocuments = new Map<string, string>();
  for (const file of readdirSync(agentDirectory).filter((name) =>
    name.endsWith(".md"),
  )) {
    const name = file.slice(0, -3);
    const document = readFileSync(join(agentDirectory, file), "utf8");
    const parsed = parseAgentDocument(document, `.opencode/agent/${file}`);
    agents.set(name, parsed.frontmatter);
    agentDocuments.set(name, document);
  }

  return {
    config: readJson(join(root, "opencode.json")),
    mise: Bun.TOML.parse(
      readFileSync(join(root, "mise.toml"), "utf8"),
    ) as JsonObject,
    openspec: Bun.YAML.parse(
      readFileSync(join(root, "openspec", "config.yaml"), "utf8"),
    ) as JsonObject,
    packageJson: readJson(join(root, ".opencode", "package.json")),
    packageLock: readJson(join(root, ".opencode", "package-lock.json")),
    gitignore: readFileSync(join(root, ".opencode", ".gitignore"), "utf8"),
    agents,
    agentDocuments,
    adapters: new Set(
      ADAPTER_PATHS.filter((path) => existsSync(join(root, path))),
    ),
  };
}

function validateBashPolicy(
  label: string,
  permission: JsonObject,
  failures: string[],
): void {
  if (permission["github_*"] === "allow") {
    failures.push(`${label}: github_* must not be allowed`);
  }
  const bash = permission.bash;
  if (bash === "allow") {
    failures.push(`${label}: scalar bash allow is forbidden`);
    return;
  }
  if (!bash || typeof bash !== "object" || Array.isArray(bash)) return;
  for (const [pattern, action] of Object.entries(bash)) {
    if (pattern === "*" && action === "allow") {
      failures.push(`${label}: broad bash allow is forbidden`);
    }
    if (action === "allow" && /^git(?:\s|$)/.test(pattern)) {
      if (!SAFE_GIT_RULES.has(pattern)) {
        failures.push(
          `${label}: unsafe Git allow rule ${JSON.stringify(pattern)}`,
        );
      }
      if (/[;&|`]/.test(pattern)) {
        failures.push(`${label}: Git rule contains shell control syntax`);
      }
    }
  }
}

export function validateStatic(input: ValidationInput): string[] {
  const failures: string[] = [];
  const { config, mise, openspec, packageJson, packageLock, agents } = input;

  if (config.default_agent !== "orchestrator") {
    failures.push("opencode.json: default_agent must be orchestrator");
  }
  if (config.model !== "zai-coding-plan/glm-5.2") {
    failures.push("opencode.json: model must be zai-coding-plan/glm-5.2");
  }
  if (config.small_model !== "opencode/mimo-v2.5-free") {
    failures.push("opencode.json: small_model must be opencode/mimo-v2.5-free");
  }
  for (const name of ["build", "plan", "explore", "general"]) {
    if (object(config.agent)[name]?.disable !== true) {
      failures.push(`opencode.json: built-in ${name} must be disabled`);
    }
  }

  const rootPermission = object(config.permission);
  if (!mapEquals(object(rootPermission.bash), ROOT_BASH_RULES)) {
    failures.push(
      "opencode.json: root bash policy drifted from the shared policy",
    );
  }
  validateBashPolicy("opencode.json permission", rootPermission, failures);

  const skillEntries = Object.entries(object(rootPermission.skill));
  if (
    skillEntries.length !== APPROVED_SKILL_RULES.length ||
    skillEntries.some(
      ([key, action], index) =>
        key !== APPROVED_SKILL_RULES[index] ||
        action !== (index === 0 ? "deny" : "allow"),
    )
  ) {
    failures.push(
      "opencode.json: skill policy must be deny-first and exactly reviewed",
    );
  }

  const plugins = Array.isArray(config.plugin) ? config.plugin : [];
  if (
    plugins.length === 0 ||
    plugins.some((spec) => !pinnedPackage(String(spec)))
  ) {
    failures.push(
      "opencode.json: every local plugin must use an exact version",
    );
  }

  for (const [name, expectedMode] of REQUIRED_AGENTS) {
    const agent = agents.get(name);
    if (!agent) {
      failures.push(`agent roster: missing ${name}`);
      continue;
    }
    if (agent.mode !== expectedMode) {
      failures.push(`agent roster: ${name} must have mode ${expectedMode}`);
    }
  }
  if (agents.has("hard-problem")) {
    failures.push("agent roster: hard-problem must remain removed");
  }

  for (const [name, agent] of agents) {
    const permission = object(agent.permission);
    validateBashPolicy(`agent ${name}`, permission, failures);
    if (agent.mode === "subagent") {
      if (permission.task !== "deny") {
        failures.push(`agent ${name}: subagents must deny task delegation`);
      }
      if (permission["github_*"] !== "deny") {
        failures.push(`agent ${name}: subagents must deny github_*`);
      }
      if (permission.external_directory !== "deny") {
        failures.push(`agent ${name}: subagents must deny external_directory`);
      }
    }
    if (READ_ONLY_AGENTS.has(name) && permission.edit !== "deny") {
      failures.push(`agent ${name}: read-only role must deny edit`);
    }
    if (WRITER_AGENTS.has(name)) {
      if (permission.edit !== "allow") {
        failures.push(`agent ${name}: writer must allow edit`);
      }
      const localBash = object(permission.bash);
      for (const [pattern, action] of Object.entries(localBash)) {
        if (action === "allow") {
          failures.push(
            `agent ${name}: writer bash rules must inherit from the root policy`,
          );
        }
      }
      if (Object.keys(localBash).length !== 0) {
        failures.push(`agent ${name}: local bash policy must be empty`);
      }
    }
  }

  const verifier = agents.get("verifier");
  if (verifier) {
    const permission = object(verifier.permission);
    for (const key of [
      "edit",
      "external_directory",
      "github_*",
      "task",
      "skill",
    ]) {
      if (permission[key] !== "deny") {
        failures.push(`agent verifier: ${key} must be denied`);
      }
    }
    if (!mapEquals(object(permission.bash), VERIFIER_BASH_RULES)) {
      failures.push("agent verifier: exact-command bash policy drifted");
    }
  }

  for (const [name, agent] of agents) {
    const task = object(object(agent.permission).task);
    for (const [target, action] of Object.entries(task)) {
      if (target === "*" || action !== "allow") continue;
      if (!agents.has(target) || agents.get(target)?.mode !== "subagent") {
        failures.push(
          `agent ${name}: task target ${target} is unresolved or not a subagent`,
        );
      }
    }
    const document = input.agentDocuments.get(name) ?? "";
    for (const match of document.matchAll(/@([a-z][a-z0-9-]*)/g)) {
      if (!agents.has(match[1])) {
        failures.push(`agent ${name}: unresolved role reference @${match[1]}`);
      }
    }
    for (const alias of FORBIDDEN_ALIASES) {
      if (alias.test(document)) {
        failures.push(`agent ${name}: contains stale model or role alias`);
      }
    }
  }

  if (openspec.schema !== "spec-driven") {
    failures.push("openspec/config.yaml: schema must be spec-driven");
  }
  const context = typeof openspec.context === "string" ? openspec.context : "";
  if (
    !context.includes("AGENTS.md") ||
    !context.includes("docs/architecture/harpia-platform.md")
  ) {
    failures.push(
      "openspec/config.yaml: canonical project pointers are missing",
    );
  }
  const rules = object(openspec.rules);
  const ruleKeys = Object.keys(rules);
  const expectedRuleKeys = ["proposal", "specs", "design", "tasks"];
  if (
    ruleKeys.length !== expectedRuleKeys.length ||
    ruleKeys.some((key, index) => key !== expectedRuleKeys[index])
  ) {
    failures.push(
      "openspec/config.yaml: rules must use proposal/specs/design/tasks only",
    );
  }
  for (const [artifact, values] of Object.entries(rules)) {
    if (
      !Array.isArray(values) ||
      values.length === 0 ||
      values.some((value) => typeof value !== "string" || value.trim() === "")
    ) {
      failures.push(
        `openspec/config.yaml: ${artifact} rules must be non-empty strings`,
      );
    }
  }
  const taskRules = Array.isArray(rules.tasks) ? rules.tasks.join(" ") : "";
  for (const phrase of [
    "executor-cold",
    "exact file paths",
    "Given/When/Then",
    "verification",
  ] as const) {
    if (!taskRules.includes(phrase)) {
      failures.push(`openspec/config.yaml: tasks rule must mention ${phrase}`);
    }
  }

  for (const path of ADAPTER_PATHS) {
    if (!input.adapters.has(path))
      failures.push(`OpenSpec adapter missing: ${path}`);
  }
  if (/^(package\.json|package-lock\.json)$/m.test(input.gitignore)) {
    failures.push(".opencode/.gitignore: package metadata must be trackable");
  }

  const cliVersion = object(mise.tools).opencode;
  const openspecVersion = object(mise.tools)["npm:@fission-ai/openspec"];
  if (cliVersion !== "1.17.18")
    failures.push("mise.toml: opencode must be pinned to 1.17.18");
  if (openspecVersion !== "1.5.0")
    failures.push("mise.toml: OpenSpec must be pinned to 1.5.0");
  const packageVersion = object(packageJson.dependencies)[
    "@opencode-ai/plugin"
  ];
  const packages = object(packageLock.packages);
  const lockRootVersion = object(object(packages[""]).dependencies)[
    "@opencode-ai/plugin"
  ];
  const lockedPlugin = object(
    packages["node_modules/@opencode-ai/plugin"],
  ).version;
  const lockedSdk = object(packages["node_modules/@opencode-ai/sdk"]).version;
  if (
    !cliVersion ||
    packageVersion !== cliVersion ||
    lockRootVersion !== cliVersion ||
    lockedPlugin !== cliVersion ||
    lockedSdk !== cliVersion
  ) {
    failures.push(
      "OpenCode CLI/plugin/SDK versions must be identical and exact",
    );
  }

  return [...new Set(failures)];
}

function configuredModels(input: ValidationInput): Set<string> {
  const models = new Set<string>();
  for (const value of [input.config.model, input.config.small_model]) {
    if (typeof value === "string") models.add(value);
  }
  for (const agent of input.agents.values()) {
    if (typeof agent.model === "string") models.add(agent.model);
  }
  return models;
}

export function validateRuntime(
  input: ValidationInput,
  runner: CommandRunner,
  checkModels = false,
): string[] {
  const failures: string[] = [];
  const version = runner(["opencode", "--version"]);
  if (version.exitCode !== 0) {
    failures.push(`runtime: opencode --version exited ${version.exitCode}`);
  } else if (version.stdout.trim() !== object(input.mise.tools).opencode) {
    failures.push("runtime: installed OpenCode version differs from mise.toml");
  }

  const resolved = runner(["opencode", "debug", "config", "--pure"]);
  if (resolved.exitCode !== 0) {
    failures.push(
      `runtime: opencode debug config --pure exited ${resolved.exitCode}`,
    );
  } else {
    try {
      const config = JSON.parse(resolved.stdout);
      if (
        config.default_agent !== "orchestrator" ||
        config.model !== "zai-coding-plan/glm-5.2" ||
        config.small_model !== "opencode/mimo-v2.5-free"
      ) {
        failures.push("runtime: resolved defaults do not match project policy");
      }
      for (const name of REQUIRED_AGENTS.keys()) {
        if (!object(config.agent)[name])
          failures.push(`runtime: resolved agent ${name} is missing`);
      }
      const verifierPermission = object(
        object(config.agent).verifier?.permission,
      );
      if (
        verifierPermission.edit !== "deny" ||
        verifierPermission.task !== "deny" ||
        verifierPermission.skill !== "deny" ||
        verifierPermission["github_*"] !== "deny" ||
        verifierPermission.external_directory !== "deny" ||
        !mapEquals(object(verifierPermission.bash), VERIFIER_BASH_RULES)
      ) {
        failures.push(
          "runtime: verifier permissions are not exact and read-only",
        );
      }
      if (
        !Array.isArray(config.plugin) ||
        !config.plugin.includes("@sveltejs/opencode@0.1.9")
      ) {
        failures.push("runtime: pinned project Svelte plugin is not resolved");
      }
    } catch {
      failures.push(
        "runtime: resolved OpenCode configuration is not valid JSON",
      );
    }
  }

  const listed = runner(["opencode", "agent", "list", "--pure"]);
  if (listed.exitCode !== 0) {
    failures.push(
      `runtime: opencode agent list --pure exited ${listed.exitCode}`,
    );
  } else {
    const loaded = new Map<string, string>();
    for (const line of listed.stdout.split(/\r?\n/)) {
      const match = line.match(/^([^\s].*) \((primary|subagent|all)\)$/);
      if (match) loaded.set(match[1], match[2]);
    }
    for (const [name, mode] of REQUIRED_AGENTS) {
      if (loaded.get(name) !== mode)
        failures.push(`runtime: loaded agent ${name} (${mode}) is missing`);
    }
    for (const name of FORBIDDEN_LOADED_AGENTS) {
      if (loaded.has(name))
        failures.push(`runtime: forbidden built-in ${name} is loaded`);
    }
  }

  if (checkModels) {
    const byProvider = new Map<string, Set<string>>();
    for (const model of configuredModels(input)) {
      const separator = model.indexOf("/");
      if (separator < 1) {
        failures.push(`models: invalid provider-qualified model ${model}`);
        continue;
      }
      const provider = model.slice(0, separator);
      if (!byProvider.has(provider)) byProvider.set(provider, new Set());
      byProvider.get(provider)!.add(model);
    }
    for (const [provider, expected] of byProvider) {
      const result = runner(["opencode", "models", provider]);
      if (result.exitCode !== 0) {
        failures.push(
          `models: provider ${provider} catalog exited ${result.exitCode}`,
        );
        continue;
      }
      const available = new Set(
        result.stdout
          .split(/\r?\n/)
          .map((line) => line.trim())
          .filter(Boolean),
      );
      for (const model of expected) {
        if (!available.has(model))
          failures.push(`models: configured model ${model} is unavailable`);
      }
    }
  }

  return [...new Set(failures)];
}

function defaultRunner(root: string, checkModels: boolean): CommandRunner {
  const isolatedHome = join(tmpdir(), "harpia-opencode-validate");
  mkdirSync(isolatedHome, { recursive: true });
  return (command) => {
    const isolatedEnvironment = checkModels
      ? {}
      : {
          XDG_CONFIG_HOME: join(isolatedHome, "config"),
          XDG_DATA_HOME: join(isolatedHome, "data"),
          XDG_CACHE_HOME: join(isolatedHome, "cache"),
          XDG_STATE_HOME: join(isolatedHome, "state"),
        };
    const process = Bun.spawnSync({
      cmd: command,
      cwd: root,
      env: {
        ...Bun.env,
        NO_COLOR: "1",
        OPENCODE_DISABLE_EXTERNAL_SKILLS: "1",
        OPENCODE_DISABLE_CLAUDE_CODE_SKILLS: "1",
        ...isolatedEnvironment,
      },
      stdout: "pipe",
      stderr: "pipe",
    });
    return {
      exitCode: process.exitCode,
      stdout: process.stdout.toString(),
      stderr: process.stderr.toString(),
    };
  };
}

export function validateProject(
  root: string,
  options: { checkModels?: boolean; runner?: CommandRunner } = {},
): string[] {
  let input: ValidationInput;
  try {
    input = loadProject(root);
  } catch (error) {
    return [
      `parse: ${error instanceof Error ? error.message : "unknown project parse error"}`,
    ];
  }
  return [
    ...validateStatic(input),
    ...validateRuntime(
      input,
      options.runner ?? defaultRunner(root, options.checkModels === true),
      options.checkModels,
    ),
  ];
}

if (import.meta.main) {
  const root = process.cwd();
  const failures = validateProject(root, {
    checkModels: process.argv.includes("--models"),
  });
  if (failures.length > 0) {
    console.error(`OpenCode validation failed (${failures.length}):`);
    for (const failure of failures) console.error(`- ${failure}`);
    process.exit(1);
  }
  console.log("OpenCode validation passed.");
}
