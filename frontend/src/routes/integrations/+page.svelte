<script lang="ts">
  import {
    AlertTriangle,
    CheckCircle2,
    Link2,
    Loader2,
    Plug,
    ShieldAlert,
  } from "lucide-svelte";
  import { page } from "$app/state";
  import { resolve } from "$app/paths";
  import { canManageIntegrations, getTenant } from "$lib/auth";
  import { toUserMessage } from "$lib/connect-errors";
  import HarpyHeading from "$lib/components/ui/HarpyHeading.svelte";
  import { ConnectionStatus } from "$lib/gen/harpia/executors/v1/executors_pb";
  import {
    connectionStatusLabelKey,
    formValuesFromCard,
    loadDemoIntegrationContext,
    saveDemoIntegrationInstallation,
    validateIntegrationForm,
    type DemoIntegrationCard,
    type DemoIntegrationFormValues,
  } from "$lib/integrations/executor-installations";
  import { locale, translate } from "$lib/i18n";

  const user = $derived(page.data.user);
  const integrationAccess = $derived(canManageIntegrations(user));

  let cards = $state<DemoIntegrationCard[]>([]);
  let forms = $state<Record<string, DemoIntegrationFormValues>>({});
  let tenantId = $state<string | null>(null);
  let initialized = $state(false);
  let loading = $state(false);
  let loadError = $state<string | null>(null);
  let actionError = $state<string | null>(null);
  let savingKey = $state<string | null>(null);
  let savedKey = $state<string | null>(null);

  $effect(() => {
    if (!integrationAccess || initialized) {
      return;
    }
    initialized = true;
    void loadIntegrations();
  });

  async function loadIntegrations() {
    loading = true;
    loadError = null;
    actionError = null;

    const tenant = getTenant();
    tenantId = tenant?.id ?? null;

    if (!tenantId) {
      loading = false;
      return;
    }

    try {
      const context = await loadDemoIntegrationContext(tenantId);
      cards = context.cards;
      forms = Object.fromEntries(
        context.cards.map((card) => [card.sku.key, formValuesFromCard(card)]),
      );
    } catch (error) {
      loadError = toUserMessage(error);
    } finally {
      loading = false;
    }
  }

  async function handleSave(card: DemoIntegrationCard) {
    if (!tenantId) {
      actionError = translate("integrations.error.noTenant", $locale);
      return;
    }

    const values = formFor(card);
    const validationCode = validateIntegrationForm(card.kind, values);
    if (validationCode) {
      actionError = translate(`integrations.error.${validationCode}`, $locale);
      return;
    }

    actionError = null;
    savedKey = null;
    savingKey = card.sku.key;

    try {
      await saveDemoIntegrationInstallation({ tenantId, card, values });
      await loadIntegrations();
      savedKey = card.sku.key;
    } catch (error) {
      actionError = translate("integrations.error.saveFailed", $locale, {
        error: toUserMessage(error),
      });
    } finally {
      savingKey = null;
    }
  }

  function formFor(card: DemoIntegrationCard): DemoIntegrationFormValues {
    return forms[card.sku.key] ?? formValuesFromCard(card);
  }

  function updateForm(
    card: DemoIntegrationCard,
    patch: Partial<DemoIntegrationFormValues>,
  ) {
    forms = {
      ...forms,
      [card.sku.key]: {
        ...formFor(card),
        ...patch,
      },
    };
  }

  function inputValue(event: Event): string {
    return (event.currentTarget as HTMLInputElement).value;
  }

  function textareaValue(event: Event): string {
    return (event.currentTarget as HTMLTextAreaElement).value;
  }

  function checkedValue(event: Event): boolean {
    return (event.currentTarget as HTMLInputElement).checked;
  }

  function statusClasses(status: ConnectionStatus): string {
    switch (status) {
      case ConnectionStatus.CONNECTED:
        return "border-green-500/30 bg-green-500/10 text-green-300";
      case ConnectionStatus.ERROR:
        return "border-red-500/30 bg-red-500/10 text-red-300";
      case ConnectionStatus.CONNECTING:
        return "border-talon-gold/30 bg-talon-gold/10 text-talon-gold";
      default:
        return "border-plumage bg-obsidian text-crown-ash";
    }
  }
</script>

<div class="mx-auto max-w-5xl px-4 py-6 lg:px-6">
  {#if !integrationAccess}
    <div
      class="flex flex-col items-center justify-center rounded-lg border border-plumage bg-obsidian-light/40 px-6 py-16 text-center"
    >
      <ShieldAlert class="mb-4 size-12 text-talon-gold" />
      <HarpyHeading tag="h1" class="mb-2 text-2xl text-cream">
        {translate("integrations.accessDenied", $locale)}
      </HarpyHeading>
      <p class="max-w-md font-body text-sm text-crown-ash">
        {translate("integrations.accessDeniedDescription", $locale)}
        <span class="text-cream">{user?.role ?? "Leader"}</span>.
      </p>
      <a
        href={resolve("/")}
        class="mt-6 rounded-md border border-plumage px-4 py-2 font-body text-sm text-crown-ash transition-colors hover:border-talon-gold hover:text-talon-gold"
      >
        {translate("integrations.returnTasks", $locale)}
      </a>
    </div>
  {:else}
    <div class="mb-6">
      <HarpyHeading tag="h1" class="text-2xl text-cream">
        {translate("integrations.heading", $locale)}
      </HarpyHeading>
      <p class="mt-1 max-w-2xl font-body text-sm text-crown-ash">
        {translate("integrations.subheading", $locale)}
      </p>
    </div>

    {#if actionError}
      <div
        class="mb-4 flex items-start gap-2 rounded-md border border-red-500/30 bg-red-500/10 px-4 py-3"
      >
        <AlertTriangle class="mt-0.5 size-4 shrink-0 text-red-400" />
        <p class="font-body text-sm text-red-300">{actionError}</p>
      </div>
    {/if}

    {#if loading}
      <div
        class="flex items-center gap-2 rounded-lg border border-plumage bg-obsidian-light/60 px-4 py-5"
      >
        <Loader2 class="size-4 animate-spin text-talon-gold" />
        <p class="font-body text-sm text-crown-ash">
          {translate("integrations.loading", $locale)}
        </p>
      </div>
    {:else if !tenantId}
      <div
        class="flex items-start gap-2 rounded-md border border-talon-gold/30 bg-talon-gold/10 px-4 py-3"
      >
        <AlertTriangle class="mt-0.5 size-4 shrink-0 text-talon-gold" />
        <p class="font-body text-sm text-cream">
          {translate("integrations.noTenant", $locale)}
        </p>
      </div>
    {:else if loadError}
      <div
        class="flex items-start gap-2 rounded-md border border-red-500/30 bg-red-500/10 px-4 py-3"
      >
        <AlertTriangle class="mt-0.5 size-4 shrink-0 text-red-400" />
        <p class="font-body text-sm text-red-300">
          {translate("integrations.loadError", $locale, { error: loadError })}
        </p>
      </div>
    {:else}
      <div class="grid gap-4 md:grid-cols-2">
        {#each cards as card (card.sku.id)}
          {@const form = formFor(card)}
          <article
            class="rounded-lg border border-plumage bg-obsidian-light/60 p-5"
            data-testid={`integration-card-${card.sku.key}`}
          >
            <div class="flex items-start justify-between gap-3">
              <div class="min-w-0">
                <div class="mb-2 flex items-center gap-2">
                  {#if card.kind === "rss"}
                    <Plug class="size-4 text-talon-gold" />
                  {:else}
                    <Link2 class="size-4 text-talon-gold" />
                  {/if}
                  <HarpyHeading tag="h2" class="text-lg text-cream">
                    {card.sku.displayName}
                  </HarpyHeading>
                </div>
                <p class="font-body text-sm text-crown-ash">
                  {translate(`integrations.${card.kind}.description`, $locale)}
                </p>
              </div>
              <span
                class="inline-flex shrink-0 rounded-full border px-2 py-0.5 font-body text-xs {statusClasses(
                  card.connectionStatus,
                )}"
              >
                {translate(
                  connectionStatusLabelKey(card.connectionStatus),
                  $locale,
                )}
              </span>
            </div>

            <dl class="mt-4 grid gap-2 font-body text-xs text-crown-ash">
              <div class="flex items-center justify-between gap-3">
                <dt>{translate("integrations.sku", $locale)}</dt>
                <dd class="font-mono text-crown-ash-dark">{card.sku.key}</dd>
              </div>
              {#if card.installation}
                <div class="flex items-center justify-between gap-3">
                  <dt>{translate("integrations.installationId", $locale)}</dt>
                  <dd class="truncate font-mono text-crown-ash-dark">
                    {card.installation.id}
                  </dd>
                </div>
              {/if}
            </dl>

            {#if !card.entitlement}
              <div
                class="mt-4 flex items-start gap-2 rounded-md border border-talon-gold/30 bg-talon-gold/10 px-3 py-2"
              >
                <AlertTriangle class="mt-0.5 size-4 shrink-0 text-talon-gold" />
                <p class="font-body text-sm text-cream">
                  {translate("integrations.notEntitled", $locale)}
                </p>
              </div>
            {:else}
              <div class="mt-5 space-y-4">
                <div>
                  <label
                    for={`display-name-${card.sku.key}`}
                    class="mb-1 block font-mono text-[10px] tracking-widest text-crown-ash-dark uppercase"
                  >
                    {translate("integrations.displayName", $locale)}
                  </label>
                  <input
                    id={`display-name-${card.sku.key}`}
                    value={form.displayName}
                    oninput={(event) =>
                      updateForm(card, { displayName: inputValue(event) })}
                    class="w-full rounded-md border border-plumage bg-obsidian px-3 py-2 font-body text-sm text-cream outline-none focus:border-talon-gold"
                  />
                </div>

                {#if card.kind === "rss"}
                  <div>
                    <label
                      for={`feeds-${card.sku.key}`}
                      class="mb-1 block font-mono text-[10px] tracking-widest text-crown-ash-dark uppercase"
                    >
                      {translate("integrations.rss.feedUrls", $locale)}
                    </label>
                    <textarea
                      id={`feeds-${card.sku.key}`}
                      rows="5"
                      value={form.feedsText}
                      placeholder={translate(
                        "integrations.rss.feedUrlsPlaceholder",
                        $locale,
                      )}
                      oninput={(event) =>
                        updateForm(card, { feedsText: textareaValue(event) })}
                      class="w-full rounded-md border border-plumage bg-obsidian px-3 py-2 font-mono text-xs text-cream outline-none focus:border-talon-gold"
                    ></textarea>
                    <p class="mt-1 font-body text-xs text-crown-ash-dark">
                      {translate("integrations.rss.feedUrlsHelp", $locale)}
                    </p>
                  </div>
                {:else}
                  <div>
                    <label
                      for={`credential-${card.sku.key}`}
                      class="mb-1 block font-mono text-[10px] tracking-widest text-crown-ash-dark uppercase"
                    >
                      {translate("integrations.linkedin.credentialId", $locale)}
                    </label>
                    <input
                      id={`credential-${card.sku.key}`}
                      value={form.oauthCredentialId}
                      placeholder={translate(
                        "integrations.linkedin.credentialIdPlaceholder",
                        $locale,
                      )}
                      oninput={(event) =>
                        updateForm(card, {
                          oauthCredentialId: inputValue(event),
                        })}
                      class="w-full rounded-md border border-plumage bg-obsidian px-3 py-2 font-mono text-xs text-cream outline-none focus:border-talon-gold"
                    />
                    <p class="mt-1 font-body text-xs text-crown-ash-dark">
                      {translate(
                        "integrations.linkedin.credentialIdHelp",
                        $locale,
                      )}
                    </p>
                  </div>
                {/if}

                <label
                  class="flex items-center gap-2 font-body text-sm text-cream"
                >
                  <input
                    type="checkbox"
                    checked={form.enabled}
                    onchange={(event) =>
                      updateForm(card, { enabled: checkedValue(event) })}
                    class="size-4 rounded border-plumage bg-obsidian text-talon-gold focus:ring-talon-gold"
                  />
                  {translate("integrations.enabled", $locale)}
                </label>

                <div class="flex flex-wrap items-center gap-3">
                  <button
                    type="button"
                    onclick={() => handleSave(card)}
                    disabled={savingKey === card.sku.key}
                    data-testid={`save-integration-${card.sku.key}`}
                    class="inline-flex cursor-pointer items-center gap-2 rounded-md bg-talon-gold px-4 py-2 font-body text-sm font-medium text-obsidian transition-all hover:bg-talon-gold-bright disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    {#if savingKey === card.sku.key}
                      <Loader2 class="size-4 animate-spin" />
                      {translate("integrations.saving", $locale)}
                    {:else}
                      <CheckCircle2 class="size-4" />
                      {translate("integrations.save", $locale)}
                    {/if}
                  </button>
                  {#if savedKey === card.sku.key}
                    <p
                      class="inline-flex items-center gap-1.5 font-body text-sm text-green-300"
                      data-testid={`integration-saved-${card.sku.key}`}
                    >
                      <CheckCircle2 class="size-4" />
                      {translate("integrations.saved", $locale)}
                    </p>
                  {/if}
                </div>
              </div>
            {/if}
          </article>
        {:else}
          <div
            class="rounded-lg border border-dashed border-plumage px-6 py-12 text-center md:col-span-2"
          >
            <Plug class="mx-auto mb-3 size-10 text-talon-gold" />
            <p class="font-body text-sm text-crown-ash">
              {translate("integrations.noDemoSkus", $locale)}
            </p>
          </div>
        {/each}
      </div>
    {/if}
  {/if}
</div>
