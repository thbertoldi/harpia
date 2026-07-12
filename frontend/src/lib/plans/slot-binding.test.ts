import { create } from "@bufbuild/protobuf";
import { describe, expect, it } from "vitest";
import {
  ConnectionStatus,
  ExecutorInstallationSchema,
  ExecutorKind as CatalogExecutorKind,
  IntegrationInstallationSchema,
  AgentInstallationSchema,
} from "$lib/gen/harpia/executors/v1/executors_pb";
import {
  ExecutorKind as PlanExecutorKind,
  ExecutorRequirementSchema,
  PlanStepSchema,
  PlanTemplateSchema,
  SlotBindingSchema,
} from "$lib/gen/harpia/plans/v1/plans_pb";
import {
  mockExecutorContext,
  NEWS_TO_SOCIAL_POST_TEMPLATE,
} from "$lib/mocks/plan-catalog";
import {
  catalogExecutorKindToPlanKind,
  executorKindsMatch,
  getCompatibleInstallationsForStep,
  isInstallationCompatibleWithStep,
  planExecutorKindToCatalogKind,
  selectionsToSlotBindings,
  slotBindingsToSelections,
  validateSlotBindings,
} from "$lib/plans/slot-binding";

const RSS_STEP = NEWS_TO_SOCIAL_POST_TEMPLATE.steps[0]!;
const WRITER_STEP = NEWS_TO_SOCIAL_POST_TEMPLATE.steps[1]!;
const VOICE_STEP = NEWS_TO_SOCIAL_POST_TEMPLATE.steps[2]!;

describe("slot binding compatibility", () => {
  it("maps executor kinds between plan and catalog enums", () => {
    expect(planExecutorKindToCatalogKind(PlanExecutorKind.AGENT)).toBe(
      CatalogExecutorKind.AGENT,
    );
    expect(planExecutorKindToCatalogKind(PlanExecutorKind.INTEGRATION)).toBe(
      CatalogExecutorKind.INTEGRATION,
    );
    expect(catalogExecutorKindToPlanKind(CatalogExecutorKind.AGENT)).toBe(
      PlanExecutorKind.AGENT,
    );
  });

  it("matches plan and catalog executor kinds", () => {
    expect(
      executorKindsMatch(
        PlanExecutorKind.INTEGRATION,
        CatalogExecutorKind.INTEGRATION,
      ),
    ).toBe(true);
    expect(
      executorKindsMatch(
        PlanExecutorKind.AGENT,
        CatalogExecutorKind.INTEGRATION,
      ),
    ).toBe(false);
  });

  it("filters compatible installations by SKU key and executor kind", () => {
    const context = mockExecutorContext();
    const rssOptions = getCompatibleInstallationsForStep(RSS_STEP, context);
    const writerOptions = getCompatibleInstallationsForStep(
      WRITER_STEP,
      context,
    );

    expect(rssOptions.map((option) => option.installation.id)).toEqual([
      "inst-rss",
      "inst-rss-hn",
      "inst-rss-sports",
    ]);
    expect(writerOptions).toHaveLength(1);
    expect(writerOptions[0]?.installation.id).toBe("inst-writer");
  });

  it("rejects installations with the wrong SKU or executor kind", () => {
    const context = mockExecutorContext();
    const wrongSkuInstallation = create(ExecutorInstallationSchema, {
      id: "inst-wrong",
      tenantId: "dev",
      executorSkuId: context.skus.find(
        (sku) => sku.key === "newsletter-writer-senior",
      )!.id,
      kind: CatalogExecutorKind.AGENT,
      displayName: "Wrong binding",
      enabled: true,
      createdAt: "2026-06-02T10:00:00Z",
      updatedAt: "2026-06-02T10:00:00Z",
      detail: {
        case: "agent",
        value: create(AgentInstallationSchema, {
          manifestId: "newsletter-writer-senior",
          manifestVersion: "1.0.0",
        }),
      },
    });

    expect(
      isInstallationCompatibleWithStep(
        RSS_STEP,
        wrongSkuInstallation,
        context.skus,
        context.entitlements,
      ),
    ).toBe(false);
  });

  it("returns no compatible options when a step SKU has no installation", () => {
    const context = mockExecutorContext();
    const voiceSkuID = context.skus.find(
      (sku) => sku.key === "linkedin-voice-senior",
    )!.id;
    const contextWithoutVoice = {
      ...context,
      installations: context.installations.filter(
        (installation) => installation.executorSkuId !== voiceSkuID,
      ),
    };
    const voiceOptions = getCompatibleInstallationsForStep(
      VOICE_STEP,
      contextWithoutVoice,
    );

    expect(voiceOptions).toHaveLength(0);
  });
});

describe("slot binding validation and lock state", () => {
  it("allows partial draft bindings with warnings", () => {
    const context = mockExecutorContext();
    const bindings = selectionsToSlotBindings(
      NEWS_TO_SOCIAL_POST_TEMPLATE,
      {
        "fetch-news": "inst-rss",
        "write-draft": "inst-writer",
      },
      context,
    );

    const validation = validateSlotBindings(
      NEWS_TO_SOCIAL_POST_TEMPLATE,
      bindings,
      context,
    );

    expect(validation.canPromoteToRunnable).toBe(false);
    expect(validation.draftWarnings.length).toBeGreaterThan(0);
    expect(validation.steps.filter((step) => step.bound)).toHaveLength(2);
  });

  it("blocks runnable promotion until all steps are bound and ready", () => {
    const context = mockExecutorContext();
    const completeSelections = {
      "fetch-news": "inst-rss",
      "write-draft": "inst-writer",
      "adapt-for-linkedin": "inst-voice",
      "publish-linkedin": "inst-linkedin",
    };

    const voiceInstallation = create(ExecutorInstallationSchema, {
      id: "inst-voice",
      tenantId: "dev",
      executorSkuId: context.skus.find(
        (sku) => sku.key === "linkedin-voice-senior",
      )!.id,
      kind: CatalogExecutorKind.AGENT,
      displayName: "LinkedIn Voice",
      enabled: true,
      createdAt: "2026-06-02T10:00:00Z",
      updatedAt: "2026-06-02T10:00:00Z",
      detail: {
        case: "agent",
        value: create(AgentInstallationSchema, {
          manifestId: "linkedin-voice-senior",
          manifestVersion: "1.0.0",
        }),
      },
    });

    const linkedinReady = create(ExecutorInstallationSchema, {
      ...context.installations.find((inst) => inst.id === "inst-linkedin")!,
      detail: {
        case: "integration",
        value: create(IntegrationInstallationSchema, {
          connectionStatus: ConnectionStatus.CONNECTED,
          configJson: '{"account":"example"}',
        }),
      },
    });

    const enrichedContext = {
      ...context,
      installations: [...context.installations, voiceInstallation].map(
        (inst) => (inst.id === "inst-linkedin" ? linkedinReady : inst),
      ),
    };

    const bindings = selectionsToSlotBindings(
      NEWS_TO_SOCIAL_POST_TEMPLATE,
      completeSelections,
      enrichedContext,
    );
    const validation = validateSlotBindings(
      NEWS_TO_SOCIAL_POST_TEMPLATE,
      bindings,
      enrichedContext,
    );

    expect(validation.canPromoteToRunnable).toBe(true);
    expect(validation.draftWarnings).toHaveLength(0);
  });

  it("round-trips slot binding selections", () => {
    const bindings = [
      create(SlotBindingSchema, {
        stepKey: "fetch-news",
        executorKind: PlanExecutorKind.INTEGRATION,
        executorSkuId: "sku-rss",
        executorInstallationId: "inst-rss",
      }),
    ];

    expect(slotBindingsToSelections(bindings)).toEqual({
      "fetch-news": "inst-rss",
    });
  });

  it("marks disconnected integration bindings as not ready", () => {
    const context = mockExecutorContext();
    const bindings = selectionsToSlotBindings(
      NEWS_TO_SOCIAL_POST_TEMPLATE,
      { "publish-linkedin": "inst-linkedin" },
      context,
    );

    const validation = validateSlotBindings(
      NEWS_TO_SOCIAL_POST_TEMPLATE,
      bindings,
      context,
    );
    const publishStep = validation.steps.find(
      (step) => step.stepKey === "publish-linkedin",
    );

    expect(publishStep?.bound).toBe(true);
    expect(publishStep?.ready).toBe(false);
    expect(publishStep?.warnings.some((warning) => warning.length > 0)).toBe(
      true,
    );
  });

  it("ignores opted-out steps when deciding whether a configuration is runnable", () => {
    const context = mockExecutorContext();
    const optionalImageStep = create(PlanStepSchema, {
      key: "generate-image",
      defaultExecutorSkuKey: "rss-news-feed",
      executorRequirement: create(ExecutorRequirementSchema, {
        executorKind: PlanExecutorKind.INTEGRATION,
        optionalCapabilities: ["image-generation"],
      }),
    });
    const template = create(PlanTemplateSchema, {
      steps: [RSS_STEP, optionalImageStep],
    });
    const bindings = selectionsToSlotBindings(
      template,
      { "fetch-news": "inst-rss" },
      context,
    );

    const optedOutValidation = validateSlotBindings(
      template,
      bindings,
      context,
      [],
    );
    expect(optedOutValidation.canPromoteToRunnable).toBe(true);
    expect(
      validateSlotBindings(template, bindings, context, ["image-generation"])
        .canPromoteToRunnable,
    ).toBe(false);
  });

  it("rejects incompatible step requirements at validation time", () => {
    const incompatibleStep = create(PlanStepSchema, {
      id: "step-agent-on-integration-slot",
      key: "agent-only",
      title: "Agent only",
      description: "",
      inputArtifactTypeId: "harpia.artifacts.v1.TextDraft",
      outputArtifactTypeId: "harpia.artifacts.v1.TextDraft",
      defaultExecutorSkuKey: "rss-news-feed",
      executorRequirement: create(ExecutorRequirementSchema, {
        executorKind: PlanExecutorKind.AGENT,
      }),
    });

    const context = mockExecutorContext();
    const rssInstallation = context.installations.find(
      (inst) => inst.id === "inst-rss",
    )!;

    expect(
      isInstallationCompatibleWithStep(
        incompatibleStep,
        rssInstallation,
        context.skus,
        context.entitlements,
      ),
    ).toBe(false);
  });
});
