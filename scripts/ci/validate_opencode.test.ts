import { describe, expect, test } from "bun:test";
import { join } from "node:path";

import {
  REQUIRED_AGENTS,
  loadProject,
  parseAgentDocument,
  validateRuntime,
  validateStatic,
  type CommandRunner,
  type ValidationInput,
} from "./validate_opencode";

const ROOT = join(import.meta.dir, "..", "..");
const baseline = loadProject(ROOT);

function fixture(): ValidationInput {
  return structuredClone(baseline);
}

function has(failures: string[], text: string): boolean {
  return failures.some((failure) => failure.includes(text));
}

function configuredModels(input: ValidationInput): Set<string> {
  const models = new Set<string>([
    input.config.model,
    input.config.small_model,
  ]);
  for (const agent of input.agents.values()) models.add(agent.model);
  models.delete(undefined as unknown as string);
  return models;
}

function runtimeRunner(
  input: ValidationInput,
  calls: string[][],
  omitModel?: string,
): CommandRunner {
  const resolvedAgents = Object.fromEntries(
    [...REQUIRED_AGENTS.keys()].map((name) => {
      const agent = structuredClone(input.agents.get(name));
      if (["implementer", "terra", "scut"].includes(name)) {
        agent.permission.bash = structuredClone(input.config.permission.bash);
      }
      return [name, agent];
    }),
  );
  const headings = [...REQUIRED_AGENTS]
    .map(([name, mode]) => `${name} (${mode})`)
    .join("\n");
  const models = configuredModels(input);
  return (command) => {
    calls.push(command);
    const key = command.join(" ");
    if (key === "opencode --version") {
      return { exitCode: 0, stdout: "1.17.18\n", stderr: "" };
    }
    if (key === "opencode debug config --pure") {
      return {
        exitCode: 0,
        stdout: JSON.stringify({
          default_agent: "orchestrator",
          model: "zai-coding-plan/glm-5.2",
          small_model: "opencode/mimo-v2.5-free",
          agent: resolvedAgents,
          plugin: ["@sveltejs/opencode@0.1.9"],
        }),
        stderr: "",
      };
    }
    if (key === "opencode agent list --pure") {
      return { exitCode: 0, stdout: `${headings}\n`, stderr: "" };
    }
    if (command[1] === "models") {
      const provider = command[2];
      const stdout = [...models]
        .filter(
          (model) => model.startsWith(`${provider}/`) && model !== omitModel,
        )
        .join("\n");
      return { exitCode: 0, stdout: `${stdout}\n`, stderr: "" };
    }
    return { exitCode: 127, stdout: "", stderr: "unexpected command" };
  };
}

describe("static OpenCode policy", () => {
  test("the repository fixture is valid", () => {
    expect(validateStatic(fixture())).toEqual([]);
  });

  test("rejects wrong defaults and enabled built-ins", () => {
    const input = fixture();
    input.config.default_agent = "build";
    input.config.agent.plan.disable = false;
    const failures = validateStatic(input);
    expect(has(failures, "default_agent must be orchestrator")).toBe(true);
    expect(has(failures, "built-in plan must be disabled")).toBe(true);
  });

  test("rejects an unreviewed skill", () => {
    const input = fixture();
    input.config.permission.skill["bmad-quick-dev"] = "allow";
    expect(has(validateStatic(input), "skill policy")).toBe(true);
  });

  test("rejects a dangerous Git allow", () => {
    const input = fixture();
    input.config.permission.bash["git *"] = "allow";
    const failures = validateStatic(input);
    expect(has(failures, "root bash policy drifted")).toBe(true);
    expect(has(failures, "unsafe Git allow rule")).toBe(true);
  });

  test("rejects a catch-all moved after narrower rules", () => {
    const input = fixture();
    const bash = input.config.permission.bash;
    delete bash["*"];
    bash["*"] = "ask";
    expect(has(validateStatic(input), "root bash policy drifted")).toBe(true);
  });

  test("rejects verifier mutation and wildcard test access", () => {
    const input = fixture();
    const verifier = input.agents.get("verifier")!;
    verifier.permission.edit = "allow";
    verifier.permission.bash["cd frontend && bunx vitest run *"] = "allow";
    const failures = validateStatic(input);
    expect(has(failures, "read-only role must deny edit")).toBe(true);
    expect(has(failures, "exact-command bash policy drifted")).toBe(true);
  });

  test("rejects a writer-local bash policy that replaces shared gates", () => {
    const input = fixture();
    input.agents.get("implementer")!.permission.bash = {
      "mise run buf-generate": "allow",
    };
    expect(has(validateStatic(input), "local bash policy must be empty")).toBe(
      true,
    );
  });

  test("rejects unresolved roles and stale aliases", () => {
    const input = fixture();
    input.agentDocuments.set(
      "implementer",
      `${input.agentDocuments.get("implementer")}\nAsk @missing-role using gpt-5.5.`,
    );
    const failures = validateStatic(input);
    expect(has(failures, "unresolved role reference @missing-role")).toBe(true);
    expect(has(failures, "stale model or role alias")).toBe(true);
  });

  test("rejects invalid OpenSpec rules", () => {
    const input = fixture();
    input.openspec.rules.unknown = ["bad"];
    input.openspec.rules.tasks = ["not executor cold"];
    const failures = validateStatic(input);
    expect(has(failures, "proposal/specs/design/tasks only")).toBe(true);
    expect(has(failures, "tasks rule must mention executor-cold")).toBe(true);
  });

  test("rejects CLI and plugin version drift", () => {
    const input = fixture();
    input.packageJson.dependencies["@opencode-ai/plugin"] = "1.17.17";
    expect(has(validateStatic(input), "CLI/plugin/SDK versions")).toBe(true);
  });

  test("rejects malformed agent frontmatter", () => {
    expect(() => parseAgentDocument("not frontmatter")).toThrow(
      "missing YAML frontmatter",
    );
  });
});

describe("resolved OpenCode checks", () => {
  test("default validation never queries model catalogs", () => {
    const input = fixture();
    const calls: string[][] = [];
    expect(validateRuntime(input, runtimeRunner(input, calls), false)).toEqual(
      [],
    );
    expect(calls.some((command) => command[1] === "models")).toBe(false);
  });

  test("the opt-in path checks exact configured models", () => {
    const input = fixture();
    const calls: string[][] = [];
    const missing = input.config.small_model;
    const failures = validateRuntime(
      input,
      runtimeRunner(input, calls, missing),
      true,
    );
    expect(calls.some((command) => command[1] === "models")).toBe(true);
    expect(has(failures, `configured model ${missing} is unavailable`)).toBe(
      true,
    );
  });
});
