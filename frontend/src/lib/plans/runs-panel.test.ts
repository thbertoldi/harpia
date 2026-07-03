import { create, type MessageInitShape } from "@bufbuild/protobuf";
import { describe, expect, it } from "vitest";
import {
  PlanConfigurationSchema,
  PlanConfigurationStatus,
  PlanExecutionSchema,
  PlanExecutionStatus,
} from "$lib/gen/harpia/plans/v1/plans_pb";
import {
  canInlineEdit,
  countActiveExecutions,
  groupExecutionsByConfiguration,
} from "$lib/plans/runs-panel";

function makeConfiguration(
  id: string,
  overrides: MessageInitShape<typeof PlanConfigurationSchema> = {},
) {
  return create(PlanConfigurationSchema, {
    id,
    tenantId: "tenant-1",
    status: PlanConfigurationStatus.RUNNABLE,
    createdAt: "2026-06-15T10:00:00Z",
    updatedAt: "2026-06-15T10:00:00Z",
    ...overrides,
  });
}

function makeExecution(
  id: string,
  configurationId: string,
  overrides: MessageInitShape<typeof PlanExecutionSchema> = {},
) {
  return create(PlanExecutionSchema, {
    id,
    tenantId: "tenant-1",
    planConfigurationId: configurationId,
    status: PlanExecutionStatus.COMPLETED,
    createdAt: "2026-06-15T11:00:00Z",
    updatedAt: "2026-06-15T11:00:00Z",
    ...overrides,
  });
}

describe("groupExecutionsByConfiguration", () => {
  it("joins executions onto their configuration", () => {
    const config = makeConfiguration("config-1");
    const execution = makeExecution("exec-1", "config-1");

    const groups = groupExecutionsByConfiguration([config], [execution]);

    expect(groups).toHaveLength(1);
    expect(groups[0]?.configuration?.id).toBe("config-1");
    expect(groups[0]?.executions.map((e) => e.id)).toEqual(["exec-1"]);
  });

  it("keeps configurations that have no executions", () => {
    const config = makeConfiguration("config-1");
    const groups = groupExecutionsByConfiguration([config], []);

    expect(groups).toHaveLength(1);
    expect(groups[0]?.executions).toEqual([]);
  });

  it("surfaces executions whose configuration was not fetched as orphan", () => {
    const execution = makeExecution("exec-1", "config-missing");
    const groups = groupExecutionsByConfiguration([], [execution]);

    expect(groups).toHaveLength(1);
    expect(groups[0]?.orphan).toBe(true);
    expect(groups[0]?.configuration?.id).toBe("config-missing");
  });

  it("orders groups by newest activity first", () => {
    const olderConfig = makeConfiguration("config-old", {
      updatedAt: "2026-06-01T00:00:00Z",
    });
    const newerConfig = makeConfiguration("config-new", {
      updatedAt: "2026-06-10T00:00:00Z",
    });

    const groups = groupExecutionsByConfiguration(
      [olderConfig, newerConfig],
      [],
    );

    expect(groups.map((g) => g.configuration?.id)).toEqual([
      "config-new",
      "config-old",
    ]);
  });

  it("sorts executions newest first within a group", () => {
    const config = makeConfiguration("config-1");
    const older = makeExecution("exec-old", "config-1", {
      updatedAt: "2026-06-01T00:00:00Z",
    });
    const newer = makeExecution("exec-new", "config-1", {
      updatedAt: "2026-06-10T00:00:00Z",
    });

    const groups = groupExecutionsByConfiguration([config], [older, newer]);

    expect(groups[0]?.executions.map((e) => e.id)).toEqual([
      "exec-new",
      "exec-old",
    ]);
  });
});

describe("canInlineEdit", () => {
  it("allows inline edits for runnable plans", () => {
    expect(
      canInlineEdit(
        makeConfiguration("c", {
          status: PlanConfigurationStatus.RUNNABLE,
        }),
      ),
    ).toBe(true);
  });

  it("allows inline edits for scheduled plans", () => {
    expect(
      canInlineEdit(
        makeConfiguration("c", {
          status: PlanConfigurationStatus.SCHEDULED,
        }),
      ),
    ).toBe(true);
  });

  it("blocks inline edits for drafts", () => {
    expect(
      canInlineEdit(
        makeConfiguration("c", {
          status: PlanConfigurationStatus.DRAFT,
        }),
      ),
    ).toBe(false);
  });

  it("blocks inline edits for disabled plans", () => {
    expect(
      canInlineEdit(
        makeConfiguration("c", {
          status: PlanConfigurationStatus.DISABLED,
        }),
      ),
    ).toBe(false);
  });

  it("blocks inline edits when there is no configuration", () => {
    expect(canInlineEdit(undefined)).toBe(false);
  });
});

describe("countActiveExecutions", () => {
  it("counts pending and running executions", () => {
    const config = makeConfiguration("config-1");
    const group = {
      configuration: config,
      executions: [
        makeExecution("e1", "config-1", {
          status: PlanExecutionStatus.RUNNING,
        }),
        makeExecution("e2", "config-1", {
          status: PlanExecutionStatus.PENDING,
        }),
        makeExecution("e3", "config-1", {
          status: PlanExecutionStatus.COMPLETED,
        }),
      ],
    };

    expect(countActiveExecutions(group)).toBe(2);
  });

  it("returns zero when nothing is in flight", () => {
    const config = makeConfiguration("config-1");
    const group = {
      configuration: config,
      executions: [
        makeExecution("e1", "config-1", {
          status: PlanExecutionStatus.COMPLETED,
        }),
      ],
    };

    expect(countActiveExecutions(group)).toBe(0);
  });
});
