<script lang="ts">
  import {
    AlertTriangle,
    BookOpen,
    CheckCircle2,
    Info,
    LayoutTemplate,
    Lock,
  } from "lucide-svelte";
  import { resolve } from "$app/paths";
  import Skeleton from "$lib/components/Skeleton.svelte";
  import {
    formatPlanLockSummary,
    formatSkuLockMessage,
    loadPlanCatalog,
    type PlanCatalogEntry,
    type RequiredExecutorSku,
    type SkuLockReason,
  } from "$lib/plans/plan-catalog";
  import HarpyHeading from "$lib/components/ui/HarpyHeading.svelte";
  import { locale, translate } from "$lib/i18n";
  import { chipFlash, hoverCardLift } from "$lib/motion/transitions";

  let entries = $state<PlanCatalogEntry[]>([]);
  let loading = $state(true);
  let loadError = $state<string | null>(null);
  let dataSource = $state<"api" | "mock">("api");

  $effect(() => {
    void fetchCatalog();
  });

  async function fetchCatalog() {
    loading = true;
    loadError = null;
    try {
      const result = await loadPlanCatalog($locale);
      entries = result.entries;
      dataSource = result.source;
      if (result.error && result.source === "mock") {
        loadError = result.error;
      } else if (result.error) {
        loadError = result.error;
      }
    } catch (e) {
      loadError =
        e instanceof Error ? e.message : translate("plans.loadError", $locale);
    } finally {
      loading = false;
    }
  }

  function skuStatusLabel(reason: SkuLockReason): string {
    return translate(`plans.lock.${reason}`, $locale);
  }

  function skuStatusClass(reason: SkuLockReason): string {
    if (reason === "available")
      return "border-green-500/40 bg-green-500/10 text-green-400";
    if (reason === "missing_entitlement") {
      return "border-danger/40 bg-danger/10 text-danger";
    }
    return "border-primary/40 bg-primary/10 text-primary";
  }

  function planActionLabel(entry: PlanCatalogEntry): string {
    return entry.isLocked
      ? translate("plans.action.locked", $locale)
      : translate("plans.action.configure", $locale);
  }

  function requiredSkuSummary(sku: RequiredExecutorSku): string {
    const steps = sku.stepKeys.join(", ");
    return translate("plans.requiredSkuSteps", $locale, { steps });
  }
</script>

<div class="flex min-h-[calc(100vh-3.5rem)] flex-col">
  <div class="flex items-center justify-between px-4 py-3 lg:px-6">
    <div>
      <HarpyHeading tag="h1" class="text-2xl text-text">
        {translate("plans.heading", $locale)}
      </HarpyHeading>
      <p class="mt-1 font-body text-sm text-text-muted">
        {translate("plans.subheading", $locale)}
      </p>
    </div>
    <div class="flex items-center gap-2">
      <a
        href={resolve("/runs")}
        class="rounded-md border border-border px-3 py-1.5 font-body text-xs text-text-muted transition-colors hover:bg-surface-hover hover:text-text"
      >
        {translate("nav.runs", $locale)}
      </a>
      {#if dataSource === "api"}
        <span
          class="rounded-full border border-border px-2.5 py-1 font-mono text-[10px] tracking-wider text-text-muted uppercase"
        >
          {translate("plans.source.live", $locale)}
        </span>
      {/if}
    </div>
  </div>

  {#if loading}
    <div class="flex flex-1 flex-col gap-4 px-4 pb-6 lg:px-6">
      <div class="grid gap-4 lg:grid-cols-2 xl:grid-cols-3">
        {#each [0, 1, 2, 3, 4, 5] as i (i)}
          <div
            class="flex flex-col rounded-lg border border-border bg-surface-elevated p-4"
          >
            <Skeleton width="60%" height="1.25rem" />
            <div class="mt-3">
              <Skeleton width="100%" height="0.75rem" count={3} />
            </div>
            <div class="mt-4">
              <Skeleton width="40%" height="1.5rem" />
            </div>
          </div>
        {/each}
      </div>
    </div>
  {:else if entries.length === 0 && loadError}
    <div class="flex flex-1 items-center justify-center">
      <div class="text-center">
        <AlertTriangle class="mx-auto mb-3 size-10 text-danger" />
        <p class="font-body text-sm text-danger">
          {translate("plans.loadError", $locale)}
        </p>
        <p class="mt-1 font-mono text-xs text-text-muted">{loadError}</p>
        <button
          onclick={() => fetchCatalog()}
          in:chipFlash
          class="mt-4 cursor-pointer rounded-md border border-border px-4 py-2 font-body text-sm text-text-muted transition-colors hover:bg-surface-hover hover:text-text"
        >
          {translate("plans.retry", $locale)}
        </button>
      </div>
    </div>
  {:else if entries.length === 0}
    <div class="flex flex-1 flex-col items-center justify-center px-4">
      <LayoutTemplate class="mb-4 size-12 text-text-muted" />
      <HarpyHeading tag="h2" class="mb-2 text-center text-xl text-text">
        {translate("plans.empty.title", $locale)}
      </HarpyHeading>
      <p class="max-w-md text-center font-body text-sm text-text-muted">
        {translate("plans.empty.description", $locale)}
      </p>
    </div>
  {:else}
    {#if loadError && dataSource === "mock"}
      <div
        class="mx-4 mb-2 rounded-md border border-border bg-surface-elevated px-3 py-2 lg:mx-6"
      >
        <p class="font-mono text-xs text-text-muted">
          {translate("plans.apiFallback", $locale)}
          {loadError}
        </p>
      </div>
    {/if}

    <div
      class="grid flex-1 gap-4 px-4 pb-6 lg:grid-cols-2 lg:px-6 xl:grid-cols-3"
    >
      {#each entries as entry (entry.template.id)}
        <article
          use:hoverCardLift
          class="flex flex-col rounded-lg border border-border bg-surface-elevated p-4"
        >
          <div class="mb-3 flex items-start justify-between gap-3">
            <div>
              <div class="mb-1 flex items-center gap-2">
                <BookOpen class="size-4 text-text-muted" />
                <HarpyHeading tag="h2" class="text-lg text-text">
                  {entry.template.name}
                </HarpyHeading>
              </div>
              <p class="font-mono text-[10px] text-text-muted-dark">
                {entry.template.key}
              </p>
            </div>
            <span
              class="inline-flex shrink-0 items-center gap-1 rounded-full border px-2 py-0.5 font-mono text-[10px] tracking-wide uppercase {entry.isLocked
                ? 'border-primary/40 bg-primary/10 text-primary'
                : 'border-green-500/40 bg-green-500/10 text-green-400'}"
              title={formatPlanLockSummary(entry, $locale) ??
                translate("plans.status.ready", $locale)}
            >
              {#if entry.isLocked}
                <Lock class="size-3" />
                {translate("plans.status.locked", $locale)}
              {:else}
                <CheckCircle2 class="size-3" />
                {translate("plans.status.ready", $locale)}
              {/if}
            </span>
          </div>

          <p class="mb-3 font-body text-sm text-text-muted">
            {entry.template.description}
          </p>

          <div class="mb-4 flex flex-wrap gap-2">
            <span
              class="rounded-full border border-border px-2 py-0.5 font-mono text-[10px] text-text-muted uppercase"
            >
              {entry.template.vertical}
            </span>
            <span
              class="rounded-full border border-border px-2 py-0.5 font-mono text-[10px] text-text-muted uppercase"
            >
              {translate("plans.stepCount", $locale, {
                count: entry.template.steps.length,
              })}
            </span>
          </div>

          <div class="mb-4">
            <p
              class="mb-2 font-mono text-[10px] tracking-widest text-text-muted-dark uppercase"
            >
              {translate("plans.requiredExecutors", $locale)}
            </p>
            <ul class="space-y-2">
              {#each entry.requiredSkus as sku (sku.skuKey)}
                <li>
                  <div
                    class="rounded-md border px-3 py-2 {skuStatusClass(
                      sku.reason,
                    )}"
                    title={formatSkuLockMessage(sku, $locale) || undefined}
                  >
                    <div class="flex items-start justify-between gap-2">
                      <div>
                        <p class="font-heading text-sm font-semibold">
                          {sku.displayName}
                        </p>
                        <p class="font-mono text-[10px] opacity-80">
                          {sku.skuKey}
                        </p>
                      </div>
                      <span
                        class="font-mono text-[10px] tracking-wide uppercase"
                      >
                        {skuStatusLabel(sku.reason)}
                      </span>
                    </div>
                    <p class="mt-1 font-body text-xs opacity-90">
                      {requiredSkuSummary(sku)}
                    </p>
                    {#if sku.reason !== "available"}
                      <p class="mt-1 flex items-start gap-1 font-body text-xs">
                        <Info class="mt-0.5 size-3 shrink-0" />
                        <span>{formatSkuLockMessage(sku, $locale)}</span>
                      </p>
                    {/if}
                  </div>
                </li>
              {/each}
            </ul>
          </div>

          <div
            class="mt-auto flex items-center justify-between gap-3 border-t border-border/40 pt-3"
          >
            {#if entry.isLocked}
              <p class="font-body text-xs text-text-muted">
                {translate("plans.lockedHint", $locale)}
              </p>
              <a
                href={resolve("/admin/integrations")}
                class="shrink-0 rounded-md border border-border px-3 py-1.5 font-body text-xs text-text-muted transition-colors hover:bg-surface-hover hover:text-text"
              >
                {translate("plans.goIntegrations", $locale)}
              </a>
            {:else}
              <p class="font-body text-xs text-text-muted">
                {translate("plans.readyHint", $locale)}
              </p>
              <a
                href={resolve(
                  `/new?template=${encodeURIComponent(entry.template.id)}`,
                )}
                in:chipFlash
                class="shrink-0 rounded-md bg-primary px-3 py-1.5 font-body text-xs font-medium text-primary-foreground transition-colors hover:opacity-90"
              >
                {planActionLabel(entry)}
              </a>
            {/if}
          </div>
        </article>
      {/each}
    </div>
  {/if}
</div>
