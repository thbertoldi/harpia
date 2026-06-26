<script lang="ts">
  import {
    AlertTriangle,
    Building2,
    ChevronRight,
    CreditCard,
    Info,
    Key,
    Lock,
    ShieldCheck,
    Zap,
  } from "lucide-svelte";
  import { resolve } from "$app/paths";
  import HarpyHeading from "$lib/components/ui/HarpyHeading.svelte";
  import Skeleton from "$lib/components/Skeleton.svelte";
  import { locale, translate } from "$lib/i18n";
  import { getTenant } from "$lib/auth";
  import {
    getLLMConfigClient,
    type LLMProvider,
    type LLMProviderConfigMeta,
  } from "$lib/clients/llm-config-client";

  // ---------------------------------------------------------------------------
  // Provider catalogue (MVP list — matches agent-runtime adapters)
  // ---------------------------------------------------------------------------

  const PROVIDERS: { id: LLMProvider; label: string }[] = [
    { id: "anthropic", label: "Anthropic" },
    { id: "openai", label: "OpenAI" },
    { id: "ollama", label: "Ollama" },
  ];

  const PROVIDER_MODELS: Record<LLMProvider, string[]> = {
    anthropic: [
      "claude-opus-4-5",
      "claude-3-5-sonnet-20241022",
      "claude-3-5-haiku-20241022",
    ],
    openai: ["gpt-4o", "gpt-4o-mini", "gpt-4-turbo"],
    ollama: ["llama3:8b", "llama3:70b", "mistral:7b"],
  };

  const SETTINGS_SECTIONS = [
    {
      key: "settings.nav.tenant",
      href: "/admin/settings#tenant",
      icon: Building2,
    },
    {
      key: "settings.nav.llmProviders",
      href: "/admin/settings#llm-providers",
      icon: Key,
    },
    { key: "settings.nav.quotas", href: "/admin/settings#quotas", icon: Zap },
    {
      key: "settings.nav.billing",
      href: "/admin/settings#billing",
      icon: CreditCard,
    },
  ] as const;

  // ---------------------------------------------------------------------------
  // State
  // ---------------------------------------------------------------------------

  const client = getLLMConfigClient();
  const tenant = getTenant();

  // LLM provider configs from the mock client
  let providerConfigs = $state<Record<LLMProvider, LLMProviderConfigMeta>>({
    anthropic: {
      provider: "anthropic",
      default_model: "claude-3-5-sonnet-20241022",
      allowed_models: [],
      has_key: false,
      managed_by: "platform",
      last_rotated_at: null,
    },
    openai: {
      provider: "openai",
      default_model: "gpt-4o",
      allowed_models: [],
      has_key: false,
      managed_by: "platform",
      last_rotated_at: null,
    },
    ollama: {
      provider: "ollama",
      default_model: "llama3:8b",
      allowed_models: [],
      has_key: false,
      managed_by: "platform",
      last_rotated_at: null,
    },
  });

  let loadingProviders = $state(true);
  let loadError = $state<string | null>(null);

  // Per-provider form state
  let keyInputs = $state<Record<LLMProvider, string>>({
    anthropic: "",
    openai: "",
    ollama: "",
  });
  let selectedModels = $state<Record<LLMProvider, string>>({
    anthropic: "claude-3-5-sonnet-20241022",
    openai: "gpt-4o",
    ollama: "llama3:8b",
  });
  let allowedModels = $state<Record<LLMProvider, string[]>>({
    anthropic: [],
    openai: [],
    ollama: [],
  });
  let savingProvider = $state<LLMProvider | null>(null);
  let removingProvider = $state<LLMProvider | null>(null);
  let confirmRemoveProvider = $state<LLMProvider | null>(null);
  let toast = $state<string | null>(null);
  let toastTimer: ReturnType<typeof setTimeout> | null = null;
  let providerErrors = $state<Partial<Record<LLMProvider, string>>>({});

  // ---------------------------------------------------------------------------
  // Load
  // ---------------------------------------------------------------------------

  $effect(() => {
    void loadProviders();
  });

  async function loadProviders() {
    loadingProviders = true;
    loadError = null;
    try {
      const res = await client.getLLMProviderConfigs({});
      for (const cfg of res.configs) {
        providerConfigs[cfg.provider] = cfg;
        selectedModels[cfg.provider] =
          cfg.default_model ?? PROVIDER_MODELS[cfg.provider][0];
        allowedModels[cfg.provider] = cfg.allowed_models ?? [];
      }
    } catch (e) {
      loadError =
        e instanceof Error
          ? e.message
          : translate("settings.llmProviders.error.unknown", $locale);
    } finally {
      loadingProviders = false;
    }
  }

  // ---------------------------------------------------------------------------
  // Helpers
  // ---------------------------------------------------------------------------

  function showToast(msg: string) {
    toast = msg;
    if (toastTimer) clearTimeout(toastTimer);
    toastTimer = setTimeout(() => {
      toast = null;
    }, 4000);
  }

  function mapErrorCode(err: unknown): string {
    const msg = err instanceof Error ? err.message : String(err);
    if (msg.includes("LLM_NO_PROVIDER_CONFIGURED")) {
      return translate(
        "settings.llmProviders.error.noProviderConfigured",
        $locale,
      );
    }
    if (msg.includes("LLM_PROVIDER_BLOCKED_BY_PLATFORM")) {
      return translate(
        "settings.llmProviders.error.blockedByPlatform",
        $locale,
      );
    }
    if (msg.includes("LLM_KEY_DECRYPTION_FAILED")) {
      return translate(
        "settings.llmProviders.error.keyDecryptionFailed",
        $locale,
      );
    }
    return translate("settings.llmProviders.error.unknown", $locale);
  }

  function relativeTime(isoTimestamp: string | null): string {
    if (!isoTimestamp) return "";
    const diff = Date.now() - new Date(isoTimestamp).getTime();
    const seconds = Math.floor(diff / 1000);
    if (seconds < 60) return translate("common.time.justNow", $locale);
    const minutes = Math.floor(seconds / 60);
    if (minutes < 60)
      return translate("common.time.minutesAgo", $locale, { count: minutes });
    const hours = Math.floor(minutes / 60);
    if (hours < 24)
      return translate("common.time.hoursAgo", $locale, { count: hours });
    const days = Math.floor(hours / 24);
    return translate("common.time.daysAgo", $locale, { count: days });
  }

  function maskedKeyLabel(cfg: LLMProviderConfigMeta): string {
    if (!cfg.has_key)
      return translate("settings.llmProviders.keyInput.neverSet", $locale);
    const ago = relativeTime(cfg.last_rotated_at);
    if (!ago || ago === translate("common.time.justNow", $locale)) {
      return translate("settings.llmProviders.keyInput.maskedJustNow", $locale);
    }
    return translate("settings.llmProviders.keyInput.masked", $locale, {
      daysAgo: ago,
    });
  }

  function badgeForConfig(cfg: LLMProviderConfigMeta): {
    label: string;
    classes: string;
  } {
    if (!cfg.has_key && cfg.managed_by === "platform") {
      return {
        label: translate("settings.llmProviders.badge.platformKey", $locale),
        classes: "border-plumage text-crown-ash",
      };
    }
    if (cfg.has_key && cfg.managed_by === "tenant_self") {
      return {
        label: translate("settings.llmProviders.badge.tenantKey", $locale),
        classes: "border-talon-gold text-talon-gold",
      };
    }
    return {
      label: translate("settings.llmProviders.badge.blocked", $locale),
      classes: "border-red-500 text-red-400",
    };
  }

  function toggleAllowedModel(provider: LLMProvider, model: string) {
    const current = allowedModels[provider];
    if (current.includes(model)) {
      allowedModels[provider] = current.filter((m) => m !== model);
    } else {
      allowedModels[provider] = [...current, model];
    }
  }

  // ---------------------------------------------------------------------------
  // Actions
  // ---------------------------------------------------------------------------

  async function saveKey(provider: LLMProvider) {
    const key = keyInputs[provider].trim();
    if (!key) return;
    savingProvider = provider;
    providerErrors[provider] = undefined;
    try {
      const res = await client.setLLMProviderConfig({
        provider,
        api_key: key,
        default_model: selectedModels[provider],
        allowed_models: allowedModels[provider],
      });
      providerConfigs[provider] = res.config;
      keyInputs[provider] = "";
      showToast(translate("settings.llmProviders.toast.saved", $locale));
    } catch (e) {
      providerErrors[provider] = mapErrorCode(e);
    } finally {
      savingProvider = null;
    }
  }

  async function rotateKey(provider: LLMProvider) {
    const key = keyInputs[provider].trim();
    if (!key) return;
    savingProvider = provider;
    providerErrors[provider] = undefined;
    try {
      const res = await client.rotateLLMProviderConfigKey({
        provider,
        new_api_key: key,
      });
      providerConfigs[provider] = res.config;
      keyInputs[provider] = "";
      showToast(translate("settings.llmProviders.toast.rotated", $locale));
    } catch (e) {
      providerErrors[provider] = mapErrorCode(e);
    } finally {
      savingProvider = null;
    }
  }

  async function removeKey(provider: LLMProvider) {
    confirmRemoveProvider = null;
    removingProvider = provider;
    providerErrors[provider] = undefined;
    try {
      await client.deleteLLMProviderConfig({ provider });
      const res = await client.getLLMProviderConfigs({});
      for (const cfg of res.configs) {
        if (cfg.provider === provider) {
          providerConfigs[provider] = cfg;
        }
      }
      keyInputs[provider] = "";
      showToast(translate("settings.llmProviders.toast.removed", $locale));
    } catch (e) {
      providerErrors[provider] = mapErrorCode(e);
    } finally {
      removingProvider = null;
    }
  }
</script>

<div class="min-h-[calc(100vh-3.5rem)] px-4 py-8 lg:px-8">
  <!-- Page header -->
  <div class="mb-8">
    <HarpyHeading tag="h1" class="text-2xl text-cream">
      {translate("settings.heading", $locale)}
    </HarpyHeading>
    <p class="mt-1 font-body text-sm text-crown-ash">
      {translate("settings.subheading", $locale)}
    </p>
  </div>

  <!-- Section nav (anchor links) -->
  <nav
    class="mb-8 flex flex-wrap gap-2"
    aria-label={translate("settings.nav.ariaLabel", $locale)}
  >
    {#each SETTINGS_SECTIONS as section (section.href)}
      <a
        href={resolve(section.href)}
        class="flex items-center gap-1.5 rounded-md border border-plumage px-3 py-1.5 font-body text-sm text-crown-ash transition-colors hover:border-talon-gold hover:text-talon-gold"
      >
        <section.icon class="size-3.5" />
        {translate(section.key, $locale)}
        <ChevronRight class="size-3 opacity-50" />
      </a>
    {/each}
  </nav>

  <div class="mx-auto max-w-3xl space-y-12">
    <!-- ======================================================================
         TENANT SECTION
         ====================================================================== -->
    <section id="tenant">
      <div class="mb-4 flex items-center gap-2">
        <Building2 class="size-5 text-talon-gold" />
        <HarpyHeading tag="h2" class="text-xl text-cream">
          {translate("settings.tenant.heading", $locale)}
        </HarpyHeading>
      </div>
      <p class="mb-4 font-body text-sm text-crown-ash">
        {translate("settings.tenant.subheading", $locale)}
      </p>

      <div class="rounded-lg border border-plumage bg-obsidian-light p-6">
        <!-- Managed-by-platform notice -->
        <div
          class="mb-5 flex items-start gap-2 rounded-md border border-talon-gold/30 bg-talon-gold/5 px-4 py-3"
        >
          <Info class="mt-0.5 size-4 shrink-0 text-talon-gold" />
          <div>
            <p class="font-body text-sm font-medium text-talon-gold">
              {translate("settings.tenant.managedByPlatform", $locale)}
            </p>
            <p class="mt-0.5 font-body text-xs text-crown-ash">
              {translate("settings.tenant.managedByPlatformHint", $locale)}
            </p>
          </div>
        </div>

        <div class="space-y-4">
          <!-- Name field (read-only) -->
          <div>
            <label
              for="tenant-name"
              class="mb-1 block font-mono text-xs tracking-wider text-crown-ash-dark uppercase"
            >
              {translate("settings.tenant.name", $locale)}
            </label>
            <input
              id="tenant-name"
              type="text"
              value={tenant?.name ?? ""}
              readonly
              disabled
              class="w-full cursor-not-allowed rounded-md border border-plumage bg-obsidian px-3 py-2 font-body text-sm text-crown-ash opacity-60"
            />
          </div>

          <!-- Slug field (read-only) -->
          <div>
            <label
              for="tenant-slug"
              class="mb-1 block font-mono text-xs tracking-wider text-crown-ash-dark uppercase"
            >
              {translate("settings.tenant.slug", $locale)}
            </label>
            <input
              id="tenant-slug"
              type="text"
              value={tenant?.id ?? ""}
              readonly
              disabled
              class="w-full cursor-not-allowed rounded-md border border-plumage bg-obsidian px-3 py-2 font-mono text-sm text-crown-ash opacity-60"
            />
            <p class="mt-1 font-body text-xs text-crown-ash-dark">
              {translate("settings.tenant.slugHint", $locale)}
            </p>
          </div>
        </div>

        <p class="mt-5 font-mono text-[10px] text-crown-ash-dark">
          {translate("settings.tenant.todoRef", $locale)}
        </p>
      </div>
    </section>

    <!-- ======================================================================
         LLM PROVIDERS SECTION
         ====================================================================== -->
    <section id="llm-providers">
      <div class="mb-4 flex items-center gap-2">
        <Key class="size-5 text-talon-gold" />
        <HarpyHeading tag="h2" class="text-xl text-cream">
          {translate("settings.llmProviders.heading", $locale)}
        </HarpyHeading>
      </div>
      <p class="mb-4 font-body text-sm text-crown-ash">
        {translate("settings.llmProviders.subheading", $locale)}
      </p>

      {#if loadingProviders}
        <div class="rounded-lg border border-border bg-surface-elevated p-6">
          <Skeleton width="60%" height="1rem" count={4} />
        </div>
      {:else if loadError}
        <div class="rounded-lg border border-danger/30 bg-danger/5 p-6">
          <div class="flex items-center gap-2 text-danger">
            <AlertTriangle class="size-4" />
            <p class="font-body text-sm">{loadError}</p>
          </div>
        </div>
      {:else}
        <!-- Empty state: all on platform keys -->
        {#if PROVIDERS.every((p) => !providerConfigs[p.id].has_key)}
          <div
            class="mb-6 rounded-lg border border-plumage bg-obsidian-light/40 p-5"
          >
            <p class="font-body text-sm font-medium text-cream">
              {translate("settings.llmProviders.empty.description", $locale)}
            </p>
            <p class="mt-1 font-body text-xs text-crown-ash">
              {translate(
                "settings.llmProviders.empty.fallbackExplain",
                $locale,
              )}
            </p>
          </div>
        {/if}

        <!-- Per-provider cards -->
        <div class="space-y-6">
          {#each PROVIDERS as provider (provider.id)}
            {@const cfg = providerConfigs[provider.id]}
            {@const badge = badgeForConfig(cfg)}
            {@const isSaving = savingProvider === provider.id}
            {@const isRemoving = removingProvider === provider.id}
            {@const providerError = providerErrors[provider.id]}

            <div
              class="rounded-lg border border-plumage bg-obsidian-light"
              data-testid={`provider-card-${provider.id}`}
            >
              <!-- Card header -->
              <div
                class="flex items-center justify-between border-b border-plumage px-6 py-4"
              >
                <div class="flex items-center gap-3">
                  <p class="font-heading text-base font-semibold text-cream">
                    {provider.label}
                  </p>
                  <span class="font-mono text-[10px] text-crown-ash-dark"
                    >{provider.id}</span
                  >
                </div>
                <span
                  class="rounded-full border px-2.5 py-1 font-mono text-[10px] tracking-wider uppercase {badge.classes}"
                  data-testid={`provider-badge-${provider.id}`}
                >
                  {badge.label}
                </span>
              </div>

              <div class="space-y-5 px-6 py-5">
                <!-- Key status / input -->
                <div>
                  <p
                    class="mb-1.5 font-mono text-xs tracking-wider text-crown-ash-dark uppercase"
                  >
                    {translate("settings.llmProviders.keyInput.label", $locale)}
                  </p>

                  {#if cfg.has_key}
                    <p
                      class="mb-2 font-mono text-xs text-crown-ash"
                      data-testid={`key-masked-${provider.id}`}
                    >
                      {maskedKeyLabel(cfg)}
                    </p>
                  {:else}
                    <p
                      class="mb-2 font-body text-xs text-crown-ash"
                      data-testid={`key-not-set-${provider.id}`}
                    >
                      {translate(
                        "settings.llmProviders.keyInput.neverSet",
                        $locale,
                      )}
                    </p>
                  {/if}

                  <input
                    type="password"
                    autocomplete="new-password"
                    placeholder={translate(
                      "settings.llmProviders.keyInput.placeholder",
                      $locale,
                    )}
                    bind:value={keyInputs[provider.id]}
                    disabled={isSaving || isRemoving}
                    class="w-full rounded-md border border-plumage bg-obsidian px-3 py-2 font-mono text-sm text-cream placeholder:text-crown-ash-dark focus:border-talon-gold focus:outline-none disabled:opacity-50"
                    data-testid={`key-input-${provider.id}`}
                  />
                </div>

                <!-- Provider error -->
                {#if providerError}
                  <div
                    class="flex items-center gap-2 rounded-md border border-red-500/30 bg-red-500/5 px-3 py-2"
                  >
                    <AlertTriangle class="size-3.5 shrink-0 text-red-400" />
                    <p
                      class="font-body text-xs text-red-400"
                      data-testid={`provider-error-${provider.id}`}
                    >
                      {providerError}
                    </p>
                  </div>
                {/if}

                <!-- Default model -->
                <div>
                  <label
                    for={`default-model-${provider.id}`}
                    class="mb-1.5 block font-mono text-xs tracking-wider text-crown-ash-dark uppercase"
                  >
                    {translate(
                      "settings.llmProviders.defaultModel.label",
                      $locale,
                    )}
                  </label>
                  <select
                    id={`default-model-${provider.id}`}
                    bind:value={selectedModels[provider.id]}
                    disabled={isSaving || isRemoving}
                    class="w-full rounded-md border border-plumage bg-obsidian px-3 py-2 font-body text-sm text-cream focus:border-talon-gold focus:outline-none disabled:opacity-50"
                  >
                    {#each PROVIDER_MODELS[provider.id] as model (model)}
                      <option value={model}>{model}</option>
                    {/each}
                  </select>
                </div>

                <!-- Allowed models multi-select -->
                <div>
                  <p
                    class="mb-1.5 font-mono text-xs tracking-wider text-crown-ash-dark uppercase"
                  >
                    {translate(
                      "settings.llmProviders.allowedModels.label",
                      $locale,
                    )}
                  </p>
                  <div class="flex flex-wrap gap-2">
                    {#each PROVIDER_MODELS[provider.id] as model (model)}
                      <button
                        type="button"
                        onclick={() => toggleAllowedModel(provider.id, model)}
                        disabled={isSaving || isRemoving}
                        class="rounded-md border px-2.5 py-1 font-mono text-[11px] transition-colors disabled:opacity-50 {allowedModels[
                          provider.id
                        ].includes(model)
                          ? 'border-talon-gold bg-talon-gold/10 text-talon-gold'
                          : 'border-plumage text-crown-ash hover:border-crown-ash'}"
                      >
                        {model}
                      </button>
                    {/each}
                  </div>
                  <p class="mt-1 font-body text-[11px] text-crown-ash-dark">
                    {translate(
                      "settings.llmProviders.allowedModels.hint",
                      $locale,
                    )}
                  </p>
                </div>

                <!-- Action buttons -->
                <div class="flex flex-wrap gap-2 border-t border-plumage pt-4">
                  {#if !cfg.has_key}
                    <button
                      type="button"
                      onclick={() => saveKey(provider.id)}
                      disabled={!keyInputs[provider.id].trim() ||
                        isSaving ||
                        isRemoving}
                      class="rounded-md bg-talon-gold px-4 py-2 font-body text-sm font-medium text-obsidian transition-opacity hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-40"
                      data-testid={`save-key-${provider.id}`}
                    >
                      {isSaving
                        ? translate(
                            "settings.llmProviders.actions.saving",
                            $locale,
                          )
                        : translate(
                            "settings.llmProviders.actions.save",
                            $locale,
                          )}
                    </button>
                  {:else}
                    <button
                      type="button"
                      onclick={() => rotateKey(provider.id)}
                      disabled={!keyInputs[provider.id].trim() ||
                        isSaving ||
                        isRemoving}
                      class="rounded-md bg-talon-gold px-4 py-2 font-body text-sm font-medium text-obsidian transition-opacity hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-40"
                      data-testid={`rotate-key-${provider.id}`}
                    >
                      {isSaving
                        ? translate(
                            "settings.llmProviders.actions.saving",
                            $locale,
                          )
                        : translate(
                            "settings.llmProviders.actions.rotate",
                            $locale,
                          )}
                    </button>
                    <button
                      type="button"
                      onclick={() => (confirmRemoveProvider = provider.id)}
                      disabled={isSaving || isRemoving}
                      class="rounded-md border border-red-500/40 px-4 py-2 font-body text-sm text-red-400 transition-colors hover:border-red-500 hover:bg-red-500/5 disabled:opacity-40"
                      data-testid={`remove-key-${provider.id}`}
                    >
                      {isRemoving
                        ? translate(
                            "settings.llmProviders.actions.removing",
                            $locale,
                          )
                        : translate(
                            "settings.llmProviders.actions.remove",
                            $locale,
                          )}
                    </button>
                  {/if}
                </div>

                <!-- Transparency footer -->
                <div class="flex items-start gap-2">
                  <ShieldCheck
                    class="mt-0.5 size-3.5 shrink-0 text-crown-ash-dark"
                  />
                  <p class="font-body text-[11px] text-crown-ash-dark">
                    {translate("settings.llmProviders.transparency", $locale)}
                  </p>
                </div>
              </div>
            </div>
          {/each}
        </div>
      {/if}
    </section>

    <!-- ======================================================================
         QUOTAS SECTION (placeholder — #34 dependency)
         ====================================================================== -->
    <section id="quotas">
      <div class="mb-4 flex items-center gap-2">
        <Zap class="size-5 text-talon-gold" />
        <HarpyHeading tag="h2" class="text-xl text-cream">
          {translate("settings.quotas.heading", $locale)}
        </HarpyHeading>
      </div>
      <p class="mb-4 font-body text-sm text-crown-ash">
        {translate("settings.quotas.subheading", $locale)}
      </p>

      <div
        class="rounded-lg border border-plumage bg-obsidian-light opacity-60"
        data-testid="quotas-section"
        aria-disabled="true"
      >
        <div class="border-b border-plumage px-6 py-4">
          <div class="flex items-center gap-2">
            <Lock class="size-4 text-crown-ash" />
            <p class="font-body text-sm font-medium text-cream">
              {translate("settings.quotas.comingSoon", $locale)}
            </p>
          </div>
          <p
            class="mt-1 font-body text-xs text-crown-ash"
            data-testid="quotas-coming-soon-text"
          >
            {translate("settings.quotas.comingSoonDescription", $locale)}
          </p>
        </div>

        <div class="space-y-5 px-6 py-5">
          <!-- Monthly budget (disabled) -->
          <div>
            <label
              for="quota-monthly-budget"
              class="mb-1.5 block font-mono text-xs tracking-wider text-crown-ash-dark uppercase"
            >
              {translate("settings.quotas.monthlyBudget", $locale)}
            </label>
            <input
              id="quota-monthly-budget"
              type="number"
              disabled
              placeholder="—"
              class="w-full cursor-not-allowed rounded-md border border-plumage bg-obsidian px-3 py-2 font-body text-sm text-crown-ash opacity-50 placeholder:text-crown-ash-dark"
            />
          </div>

          <!-- Hard cap toggle (disabled) -->
          <div class="flex items-center gap-3">
            <input
              id="quota-hard-cap"
              type="checkbox"
              disabled
              class="cursor-not-allowed opacity-50"
            />
            <label
              for="quota-hard-cap"
              class="font-body text-sm text-crown-ash opacity-50"
            >
              {translate("settings.quotas.hardCap", $locale)}
            </label>
          </div>

          <!-- Usage gauge (disabled — shows em-dash per design spec) -->
          <div>
            <p
              class="mb-1.5 font-mono text-xs tracking-wider text-crown-ash-dark uppercase"
            >
              {translate("settings.quotas.usageGauge", $locale)}
            </p>
            <div class="flex items-center gap-3">
              <div class="h-2 flex-1 rounded-full bg-plumage">
                <div class="h-2 w-0 rounded-full bg-crown-ash-dark"></div>
              </div>
              <span class="font-mono text-xs text-crown-ash-dark">
                {translate("common.emDash", $locale)}
              </span>
            </div>
          </div>
        </div>
      </div>
    </section>

    <!-- ======================================================================
         BILLING SECTION (placeholder)
         ====================================================================== -->
    <section id="billing">
      <div class="mb-4 flex items-center gap-2">
        <CreditCard class="size-5 text-talon-gold" />
        <HarpyHeading tag="h2" class="text-xl text-cream">
          {translate("settings.billing.heading", $locale)}
        </HarpyHeading>
      </div>

      <div
        class="rounded-lg border border-plumage bg-obsidian-light px-6 py-8 text-center"
      >
        <CreditCard class="mx-auto mb-3 size-8 text-crown-ash-dark" />
        <p
          class="font-body text-sm text-crown-ash"
          data-testid="billing-placeholder"
        >
          {translate("settings.billing.placeholder", $locale)}
        </p>
      </div>
    </section>
  </div>
</div>

<!-- Key deletion confirmation dialog -->
{#if confirmRemoveProvider}
  {@const providerLabel =
    PROVIDERS.find((p) => p.id === confirmRemoveProvider)?.label ??
    confirmRemoveProvider}
  <div
    class="fixed inset-0 z-50 flex items-center justify-center bg-obsidian/70 backdrop-blur-sm"
    role="dialog"
    aria-modal="true"
  >
    <div
      class="w-full max-w-md rounded-lg border border-plumage bg-obsidian-light p-6 shadow-xl"
    >
      <p class="mb-2 font-heading text-base font-semibold text-cream">
        {translate("settings.llmProviders.removeConfirm.title", $locale)} ({providerLabel})
      </p>
      <p class="mb-6 font-body text-sm text-crown-ash">
        {translate("settings.llmProviders.removeConfirm.description", $locale)}
      </p>
      <div class="flex gap-3">
        <button
          type="button"
          onclick={() => (confirmRemoveProvider = null)}
          class="flex-1 rounded-md border border-plumage px-4 py-2 font-body text-sm text-crown-ash transition-colors hover:border-talon-gold hover:text-cream"
        >
          {translate("settings.llmProviders.removeConfirm.cancel", $locale)}
        </button>
        <button
          type="button"
          onclick={() =>
            confirmRemoveProvider && removeKey(confirmRemoveProvider)}
          class="flex-1 rounded-md bg-red-600 px-4 py-2 font-body text-sm font-medium text-cream transition-opacity hover:opacity-90"
          data-testid="confirm-remove-btn"
        >
          {translate("settings.llmProviders.removeConfirm.confirm", $locale)}
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Toast notification -->
{#if toast}
  <div
    class="fixed right-6 bottom-6 z-50 max-w-sm rounded-lg border border-plumage bg-obsidian-light px-4 py-3 shadow-lg"
    role="status"
    data-testid="toast"
  >
    <p class="font-body text-sm text-cream">{toast}</p>
  </div>
{/if}
