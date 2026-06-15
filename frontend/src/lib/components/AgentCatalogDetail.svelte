<script lang="ts">
  import {
    ArrowUpCircle,
    ExternalLink,
    Eye,
    EyeOff,
    FlaskConical,
    History,
    Link2,
    Loader2,
    RotateCcw,
    Trash2,
    X,
  } from "lucide-svelte";
  import HarpyHeading from "$lib/components/ui/HarpyHeading.svelte";
  import {
    bindToolToAgentCatalogEntry,
    runTestInvocationForAgent,
    type AgentCatalogEntry,
    type AgentCatalogSource,
  } from "$lib/agent-catalog";
  import { locale, translate } from "$lib/i18n";
  import { formatLocaleDateTime } from "$lib/i18n/format";
  import type { BoundTool } from "$lib/mocks/agent-catalog";

  let {
    entry,
    source,
    discoveredTools,
    onclose,
    onAction,
  }: {
    entry: AgentCatalogEntry;
    source: AgentCatalogSource;
    discoveredTools: BoundTool[];
    onclose: () => void;
    onAction: (action: string, message: string) => void;
  } = $props();
  let selectedToolId = $state<string>("");
  let bindingTool = $state(false);
  let invoking = $state(false);
  let actionError = $state<string | null>(null);

  const bindableTools = $derived(
    discoveredTools.filter(
      (tool) => !entry.boundTools.some((boundTool) => boundTool.id === tool.id),
    ),
  );

  $effect(() => {
    if (
      bindableTools.length > 0 &&
      !bindableTools.some((tool) => tool.id === selectedToolId)
    ) {
      selectedToolId = bindableTools[0].id;
    }
    if (bindableTools.length === 0) {
      selectedToolId = "";
    }
  });

  function confirmAction(
    action: string,
    message: string,
    prompt: string,
  ): void {
    if (window.confirm(prompt)) {
      onAction(action, message);
    }
  }

  function formattedDate(dateStr: string): string {
    try {
      return formatLocaleDateTime(dateStr, $locale, {
        month: "short",
        day: "numeric",
        hour: "2-digit",
        minute: "2-digit",
      });
    } catch {
      return dateStr;
    }
  }

  function invocationStatusClass(status: string): string {
    switch (status) {
      case "completed":
        return "text-green-400 bg-green-500/10";
      case "failed":
        return "text-red-400 bg-red-500/10";
      default:
        return "text-talon-gold bg-talon-gold/10";
    }
  }

  function versionStatusClass(status: string): string {
    switch (status) {
      case "active":
        return "text-green-400";
      case "canary":
        return "text-talon-gold";
      case "deprecated":
        return "text-red-400";
      default:
        return "text-crown-ash";
    }
  }

  async function handleBindTool(): Promise<void> {
    if (!selectedToolId) return;
    const selectedTool = bindableTools.find(
      (tool) => tool.id === selectedToolId,
    );
    if (!selectedTool) return;

    actionError = null;
    bindingTool = true;
    try {
      const result = await bindToolToAgentCatalogEntry(
        source,
        entry.agentType.id,
        selectedTool,
        $locale,
      );
      entry = result.entry;
      onAction(
        "bind-tool",
        `Bound ${selectedTool.name} to ${entry.agentType.displayName || entry.agentType.name}.`,
      );
    } catch (error) {
      actionError =
        error instanceof Error ? error.message : "Failed to bind tool.";
    } finally {
      bindingTool = false;
    }
  }

  async function handleRunTestInvocation(): Promise<void> {
    actionError = null;
    invoking = true;
    try {
      const result = await runTestInvocationForAgent(
        source,
        entry.agentType.id,
        $locale,
      );
      entry = result.entry;
      onAction(
        "test-invocation",
        `Test invocation completed for task ${result.taskId}.`,
      );
    } catch (error) {
      actionError =
        error instanceof Error
          ? error.message
          : "Failed to run test invocation.";
    } finally {
      invoking = false;
    }
  }
</script>

<div class="flex h-full flex-col">
  <div
    class="flex items-start justify-between gap-3 border-b border-plumage px-4 py-3"
  >
    <div class="min-w-0">
      <HarpyHeading tag="h2" class="truncate text-xl text-cream">
        {entry.agentType.displayName || entry.agentType.name}
      </HarpyHeading>
      <p
        class="mt-0.5 font-mono text-[10px] tracking-wider text-crown-ash-dark"
      >
        {entry.agentType.id}
      </p>
    </div>
    <button
      onclick={onclose}
      class="cursor-pointer rounded-md p-1 text-crown-ash transition-colors hover:text-cream"
      aria-label={translate("agents.closeDetails", $locale)}
    >
      <X class="size-4" />
    </button>
  </div>

  <div class="flex-1 space-y-6 overflow-y-auto p-4">
    <section>
      <h3
        class="mb-2 font-mono text-[10px] tracking-widest text-crown-ash-dark uppercase"
      >
        {translate("agents.detail.actions", $locale)}
      </h3>
      {#if actionError}
        <p class="mb-2 font-body text-xs text-red-400">{actionError}</p>
      {/if}
      <div class="flex flex-wrap gap-2">
        {#if bindableTools.length > 0}
          <div
            class="inline-flex items-center gap-2 rounded-md border border-plumage px-2 py-1"
          >
            <select
              bind:value={selectedToolId}
              class="max-w-40 bg-transparent font-mono text-[10px] text-crown-ash outline-none"
              data-testid="agent-bind-tool-select"
            >
              {#each bindableTools as tool (tool.id)}
                <option value={tool.id}>{tool.name}</option>
              {/each}
            </select>
            <button
              type="button"
              onclick={handleBindTool}
              disabled={bindingTool || invoking}
              class="inline-flex cursor-pointer items-center gap-1 rounded-md border border-plumage px-2 py-1 font-body text-[11px] text-cream transition-colors hover:border-talon-gold hover:text-talon-gold disabled:cursor-not-allowed disabled:opacity-50"
              data-testid="agent-bind-selected-tool"
            >
              {#if bindingTool}
                <Loader2 class="size-3 animate-spin" />
              {:else}
                <Link2 class="size-3" />
              {/if}
              {translate("integrations.bindToAgent", $locale)}
            </button>
          </div>
        {/if}
        <button
          type="button"
          onclick={handleRunTestInvocation}
          disabled={bindingTool || invoking}
          class="inline-flex cursor-pointer items-center gap-1.5 rounded-md border border-plumage px-3 py-1.5 font-body text-xs text-cream transition-colors hover:border-talon-gold hover:text-talon-gold disabled:cursor-not-allowed disabled:opacity-50"
          data-testid="agent-run-test-invocation"
        >
          {#if invoking}
            <Loader2 class="size-3.5 animate-spin" />
          {:else}
            <FlaskConical class="size-3.5" />
          {/if}
          {translate("agents.detail.runTestInvocation", $locale)}
        </button>
        {#if entry.canaryVersion}
          <button
            onclick={() =>
              confirmAction(
                "promote-canary",
                `Promoted ${entry.canaryVersion} to active (stub)`,
                `Promote canary ${entry.canaryVersion} to active for ${entry.agentType.displayName}?`,
              )}
            class="inline-flex cursor-pointer items-center gap-1.5 rounded-md border border-plumage px-3 py-1.5 font-body text-xs text-cream transition-colors hover:border-talon-gold hover:text-talon-gold"
          >
            <ArrowUpCircle class="size-3.5" />
            {translate("agents.detail.promoteCanary", $locale)}
          </button>
        {/if}
        <button
          onclick={() =>
            confirmAction(
              "rollback",
              `Rolled back to previous version (stub)`,
              `Roll back ${entry.agentType.displayName} to the previous active version?`,
            )}
          class="inline-flex cursor-pointer items-center gap-1.5 rounded-md border border-plumage px-3 py-1.5 font-body text-xs text-cream transition-colors hover:border-talon-gold hover:text-talon-gold"
        >
          <RotateCcw class="size-3.5" />
          {translate("agents.detail.rollback", $locale)}
        </button>
        <button
          onclick={() =>
            onAction(
              "edit-visibility",
              `Visibility editor opened (stub) — current: ${entry.tenantVisibility}`,
            )}
          class="inline-flex cursor-pointer items-center gap-1.5 rounded-md border border-plumage px-3 py-1.5 font-body text-xs text-cream transition-colors hover:border-talon-gold hover:text-talon-gold"
        >
          {#if entry.tenantVisibility === "global"}
            <Eye class="size-3.5" />
          {:else}
            <EyeOff class="size-3.5" />
          {/if}
          {translate("agents.detail.editVisibility", $locale)}
        </button>
        <button
          onclick={() =>
            confirmAction(
              "deprecate",
              `${entry.agentType.displayName} marked deprecated (stub)`,
              `Deprecate ${entry.agentType.displayName}? Existing invocations may still complete.`,
            )}
          class="inline-flex cursor-pointer items-center gap-1.5 rounded-md border border-red-500/30 px-3 py-1.5 font-body text-xs text-red-400 transition-colors hover:bg-red-500/10"
        >
          <Trash2 class="size-3.5" />
          {translate("agents.detail.deprecate", $locale)}
        </button>
      </div>
    </section>

    <section>
      <div class="mb-2 flex items-center justify-between">
        <h3
          class="font-mono text-[10px] tracking-widest text-crown-ash-dark uppercase"
        >
          {translate("agents.detail.manifest", $locale)}
        </h3>
        <button
          type="button"
          onclick={() =>
            window.open(entry.yamlSourceUrl, "_blank", "noopener,noreferrer")}
          class="inline-flex cursor-pointer items-center gap-1 border-0 bg-transparent p-0 font-body text-xs text-talon-gold transition-colors hover:text-talon-gold-bright"
        >
          {translate("agents.detail.yamlSource", $locale)}
          <ExternalLink class="size-3" />
        </button>
      </div>
      <pre
        class="max-h-64 overflow-auto rounded-lg border border-plumage bg-obsidian p-3 font-mono text-[11px] leading-relaxed text-crown-ash">{JSON.stringify(
          {
            id: entry.agentType.id,
            version: entry.agentType.version,
            display_name: entry.agentType.displayName,
            description: entry.agentType.description,
            capabilities: entry.agentType.capabilities,
            model_id: entry.agentType.modelId,
            system_prompt: entry.agentType.systemPrompt,
            allowed_tool_ids: entry.agentType.allowedToolIds,
            input_schema: entry.agentType.inputSchema,
            output_schema: entry.agentType.outputSchema,
            cost_estimate: entry.agentType.costEstimate,
            metadata: entry.agentType.metadata,
          },
          null,
          2,
        )}</pre>
    </section>

    <section>
      <h3
        class="mb-2 font-mono text-[10px] tracking-widest text-crown-ash-dark uppercase"
      >
        {translate("agents.detail.boundTools", $locale)}
      </h3>
      {#if entry.boundTools.length === 0}
        <p class="font-body text-sm text-crown-ash">
          {translate("agents.detail.noTools", $locale)}
        </p>
      {:else}
        <ul class="space-y-2">
          {#each entry.boundTools as tool (tool.id)}
            <li
              class="rounded-md border border-plumage bg-obsidian-light/40 px-3 py-2"
              data-testid={`agent-bound-tool-${tool.id}`}
            >
              <p class="font-body text-sm font-medium text-cream">
                {tool.name}
              </p>
              <p class="font-mono text-[10px] text-crown-ash-dark">
                {tool.id}
              </p>
              <p class="mt-1 font-body text-xs text-crown-ash">
                {tool.description}
              </p>
            </li>
          {/each}
        </ul>
      {/if}
    </section>

    <section>
      <h3
        class="mb-2 flex items-center gap-1.5 font-mono text-[10px] tracking-widest text-crown-ash-dark uppercase"
      >
        <History class="size-3" />
        {translate("agents.detail.versionHistory", $locale)}
      </h3>
      <ul class="space-y-1.5">
        {#each entry.versionHistory as version (version.version)}
          <li
            class="flex items-center justify-between rounded-md border border-plumage/60 px-3 py-2"
          >
            <div>
              <span class="font-mono text-sm text-cream">{version.version}</span
              >
              <span
                class="ml-2 font-mono text-[10px] uppercase {versionStatusClass(
                  version.status,
                )}">{version.status}</span
              >
            </div>
            <span class="font-mono text-[10px] text-crown-ash-dark">
              {formattedDate(version.deployedAt)}
            </span>
          </li>
        {/each}
      </ul>
    </section>

    <section>
      <h3
        class="mb-2 font-mono text-[10px] tracking-widest text-crown-ash-dark uppercase"
      >
        {translate("agents.detail.recentInvocations", $locale)}
      </h3>
      {#if entry.recentInvocations.length === 0}
        <p class="font-body text-sm text-crown-ash">
          {translate("agents.detail.noInvocations", $locale)}
        </p>
      {:else}
        <ul class="space-y-2">
          {#each entry.recentInvocations as invocation (invocation.id)}
            <li
              class="flex items-center justify-between rounded-md border border-plumage/60 px-3 py-2"
              data-testid={`agent-invocation-${invocation.id}`}
            >
              <div>
                <span class="font-mono text-xs text-cream">{invocation.id}</span
                >
                <span class="ml-2 font-mono text-[10px] text-crown-ash-dark"
                  >task {invocation.taskId.slice(0, 8)}</span
                >
              </div>
              <div class="flex items-center gap-2">
                {#if invocation.durationMs}
                  <span class="font-mono text-[10px] text-crown-ash-dark"
                    >{Math.round(invocation.durationMs / 1000)}s</span
                  >
                {/if}
                <span
                  class="rounded-full px-2 py-0.5 font-mono text-[10px] tracking-wider uppercase {invocationStatusClass(
                    invocation.status,
                  )}">{invocation.status.replace("_", " ")}</span
                >
              </div>
            </li>
          {/each}
        </ul>
      {/if}
    </section>
  </div>
</div>
