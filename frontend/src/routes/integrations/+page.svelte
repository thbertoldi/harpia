<script lang="ts">
  import {
    Plus,
    Loader2,
    ChevronDown,
    ChevronRight,
    Plug,
    ShieldAlert,
    Link2,
    Wrench,
    CheckCircle2,
    AlertTriangle,
    X,
  } from "lucide-svelte";
  import { page } from "$app/state";
  import { resolve } from "$app/paths";
  import { canManageIntegrations } from "$lib/auth";
  import HarpyHeading from "$lib/components/ui/HarpyHeading.svelte";
  import McpStatusBadge from "$lib/components/McpStatusBadge.svelte";
  import { locale, translate } from "$lib/i18n";
  import {
    addMockMcpServer,
    connectMockOAuthServer,
    getMockMcpServers,
    testMockConnection,
    type AddMcpServerInput,
    type McpServer,
    type McpServerKind,
    type StreamableHttpConfig,
    type StdioConfig,
  } from "$lib/mocks/mcp-servers";

  const user = $derived(page.data.user);
  const integrationAccess = $derived(canManageIntegrations(user));

  let servers = $state<McpServer[]>([]);
  let expandedServerIds = $state<string[]>([]);
  let showAddForm = $state(false);
  let addKind = $state<McpServerKind>("stdio");
  let addName = $state("");
  let addCommand = $state("npx");
  let addArgs = $state("-y @modelcontextprotocol/server-filesystem /workspace");
  let addEnv = $state("");
  let addUrl = $state("https://");
  let addHeaders = $state("");
  let testing = $state(false);
  let testResult = $state<{ ok: boolean; message: string } | null>(null);
  let saving = $state(false);
  let connectingId = $state<string | null>(null);
  let actionError = $state<string | null>(null);
  let bindNotice = $state<string | null>(null);

  $effect(() => {
    if (integrationAccess) {
      servers = getMockMcpServers();
    }
  });

  function isExpanded(serverId: string): boolean {
    return expandedServerIds.includes(serverId);
  }

  function toggleExpanded(serverId: string) {
    if (isExpanded(serverId)) {
      expandedServerIds = expandedServerIds.filter((id) => id !== serverId);
    } else {
      expandedServerIds = [...expandedServerIds, serverId];
    }
  }

  function resetAddForm() {
    addKind = "stdio";
    addName = "";
    addCommand = "npx";
    addArgs = "-y @modelcontextprotocol/server-filesystem /workspace";
    addEnv = "";
    addUrl = "https://";
    addHeaders = "";
    testResult = null;
    actionError = null;
  }

  function openAddForm() {
    resetAddForm();
    showAddForm = true;
  }

  function closeAddForm() {
    showAddForm = false;
    testResult = null;
    actionError = null;
  }

  function buildAddInput(): AddMcpServerInput {
    if (addKind === "stdio") {
      const config: StdioConfig = {
        command: addCommand.trim(),
        args: addArgs
          .split(/\s+/)
          .map((arg) => arg.trim())
          .filter(Boolean),
      };

      const envLines = addEnv
        .split("\n")
        .map((line) => line.trim())
        .filter(Boolean);
      if (envLines.length > 0) {
        config.env = Object.fromEntries(
          envLines.map((line) => {
            const [key, ...rest] = line.split("=");
            return [key.trim(), rest.join("=").trim()];
          }),
        );
      }

      return { name: addName, kind: addKind, config };
    }

    const config: StreamableHttpConfig = {
      url: addUrl.trim(),
    };

    const headerLines = addHeaders
      .split("\n")
      .map((line) => line.trim())
      .filter(Boolean);
    if (headerLines.length > 0) {
      config.headers = Object.fromEntries(
        headerLines.map((line) => {
          const [key, ...rest] = line.split(":");
          return [key.trim(), rest.join(":").trim()];
        }),
      );
    }

    return { name: addName, kind: addKind, config };
  }

  async function handleTestConnection() {
    actionError = null;
    testResult = null;
    testing = true;
    try {
      const result = await testMockConnection(buildAddInput());
      testResult = { ok: result.ok, message: result.message };
    } catch (e) {
      testResult = {
        ok: false,
        message:
          e instanceof Error
            ? e.message
            : translate("integrations.error.connectionTestFailed", $locale),
      };
    } finally {
      testing = false;
    }
  }

  async function handleAddServer() {
    actionError = null;
    if (!addName.trim()) {
      actionError = translate("integrations.error.serverNameRequired", $locale);
      return;
    }

    saving = true;
    try {
      const created = await addMockMcpServer(buildAddInput());
      servers = getMockMcpServers();
      expandedServerIds = [...expandedServerIds, created.id];
      closeAddForm();
    } catch (e) {
      actionError =
        e instanceof Error
          ? e.message
          : translate("integrations.error.addServerFailed", $locale);
    } finally {
      saving = false;
    }
  }

  async function handleConnect(serverId: string) {
    actionError = null;
    bindNotice = null;
    connectingId = serverId;
    try {
      await connectMockOAuthServer(serverId);
      servers = getMockMcpServers();
      expandedServerIds = [...expandedServerIds, serverId];
    } catch (e) {
      actionError =
        e instanceof Error
          ? e.message
          : translate("integrations.error.oauthFailed", $locale);
    } finally {
      connectingId = null;
    }
  }

  function handleBindToAgent(server: McpServer, toolName: string) {
    bindNotice = translate("integrations.bindNotice", $locale, {
      tool: toolName,
      server: server.name,
    });
  }

  function kindLabel(kind: McpServerKind): string {
    return kind === "stdio" ? "stdio" : "streamable HTTP";
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
    <div class="mb-6 flex items-center justify-between gap-4">
      <div>
        <HarpyHeading tag="h1" class="text-2xl text-cream">
          {translate("integrations.heading", $locale)}
        </HarpyHeading>
        <p class="mt-1 font-body text-sm text-crown-ash">
          {translate("integrations.subheading", $locale)}
        </p>
      </div>
      <button
        onclick={openAddForm}
        class="inline-flex shrink-0 cursor-pointer items-center gap-2 rounded-md bg-talon-gold px-4 py-2 font-body text-sm font-medium text-obsidian transition-all hover:bg-talon-gold-bright"
      >
        <Plus class="size-4" />
        {translate("integrations.addServer", $locale)}
      </button>
    </div>

    {#if actionError}
      <div
        class="mb-4 flex items-start gap-2 rounded-md border border-red-500/30 bg-red-500/10 px-4 py-3"
      >
        <AlertTriangle class="mt-0.5 size-4 shrink-0 text-red-400" />
        <p class="font-body text-sm text-red-300">{actionError}</p>
      </div>
    {/if}

    {#if bindNotice}
      <div
        class="mb-4 flex items-start gap-2 rounded-md border border-talon-gold/30 bg-talon-gold/10 px-4 py-3"
      >
        <Link2 class="mt-0.5 size-4 shrink-0 text-talon-gold" />
        <p class="font-body text-sm text-cream">{bindNotice}</p>
      </div>
    {/if}

    {#if showAddForm}
      <section
        class="mb-6 rounded-lg border border-plumage bg-obsidian-light/60 p-5"
      >
        <div class="mb-4 flex items-center justify-between">
          <HarpyHeading tag="h2" class="text-lg text-cream">
            {translate("integrations.addMcpServer", $locale)}
          </HarpyHeading>
          <button
            onclick={closeAddForm}
            class="cursor-pointer rounded-md p-1 text-crown-ash transition-colors hover:text-cream"
            aria-label={translate("integrations.closeAddForm", $locale)}
          >
            <X class="size-4" />
          </button>
        </div>

        <div class="space-y-4">
          <div>
            <label
              for="server-name"
              class="mb-1 block font-mono text-[10px] tracking-widest text-crown-ash-dark uppercase"
            >
              {translate("integrations.serverName", $locale)}
            </label>
            <input
              id="server-name"
              bind:value={addName}
              placeholder={translate(
                "integrations.serverNamePlaceholder",
                $locale,
              )}
              class="w-full rounded-md border border-plumage bg-obsidian px-3 py-2 font-body text-sm text-cream outline-none focus:border-talon-gold"
            />
          </div>

          <div>
            <p
              class="mb-2 font-mono text-[10px] tracking-widest text-crown-ash-dark uppercase"
            >
              {translate("integrations.transportKind", $locale)}
            </p>
            <div class="flex flex-wrap gap-2">
              <button
                type="button"
                onclick={() => (addKind = "stdio")}
                class="cursor-pointer rounded-md border px-3 py-1.5 font-body text-sm transition-colors {addKind ===
                'stdio'
                  ? 'border-talon-gold bg-talon-gold/10 text-talon-gold'
                  : 'border-plumage text-crown-ash hover:border-talon-gold/50 hover:text-cream'}"
              >
                stdio
              </button>
              <button
                type="button"
                onclick={() => (addKind = "streamable_http")}
                class="cursor-pointer rounded-md border px-3 py-1.5 font-body text-sm transition-colors {addKind ===
                'streamable_http'
                  ? 'border-talon-gold bg-talon-gold/10 text-talon-gold'
                  : 'border-plumage text-crown-ash hover:border-talon-gold/50 hover:text-cream'}"
              >
                streamable HTTP
              </button>
            </div>
          </div>

          {#if addKind === "stdio"}
            <div class="grid gap-4 md:grid-cols-2">
              <div>
                <label
                  for="server-command"
                  class="mb-1 block font-mono text-[10px] tracking-widest text-crown-ash-dark uppercase"
                >
                  {translate("integrations.command", $locale)}
                </label>
                <input
                  id="server-command"
                  bind:value={addCommand}
                  class="w-full rounded-md border border-plumage bg-obsidian px-3 py-2 font-body text-sm text-cream outline-none focus:border-talon-gold"
                />
              </div>
              <div>
                <label
                  for="server-args"
                  class="mb-1 block font-mono text-[10px] tracking-widest text-crown-ash-dark uppercase"
                >
                  {translate("integrations.arguments", $locale)}
                </label>
                <input
                  id="server-args"
                  bind:value={addArgs}
                  class="w-full rounded-md border border-plumage bg-obsidian px-3 py-2 font-body text-sm text-cream outline-none focus:border-talon-gold"
                />
              </div>
            </div>
            <div>
              <label
                for="server-env"
                class="mb-1 block font-mono text-[10px] tracking-widest text-crown-ash-dark uppercase"
              >
                {translate("integrations.environment", $locale)}
              </label>
              <textarea
                id="server-env"
                bind:value={addEnv}
                rows="3"
                class="w-full rounded-md border border-plumage bg-obsidian px-3 py-2 font-mono text-xs text-cream outline-none focus:border-talon-gold"
              ></textarea>
            </div>
          {:else}
            <div>
              <label
                for="server-url"
                class="mb-1 block font-mono text-[10px] tracking-widest text-crown-ash-dark uppercase"
              >
                {translate("integrations.endpointUrl", $locale)}
              </label>
              <input
                id="server-url"
                bind:value={addUrl}
                class="w-full rounded-md border border-plumage bg-obsidian px-3 py-2 font-body text-sm text-cream outline-none focus:border-talon-gold"
              />
            </div>
            <div>
              <label
                for="server-headers"
                class="mb-1 block font-mono text-[10px] tracking-widest text-crown-ash-dark uppercase"
              >
                {translate("integrations.headers", $locale)}
              </label>
              <textarea
                id="server-headers"
                bind:value={addHeaders}
                rows="3"
                class="w-full rounded-md border border-plumage bg-obsidian px-3 py-2 font-mono text-xs text-cream outline-none focus:border-talon-gold"
              ></textarea>
            </div>
          {/if}

          {#if testResult}
            <div
              class="flex items-start gap-2 rounded-md border px-3 py-2 {testResult.ok
                ? 'border-green-500/30 bg-green-500/10'
                : 'border-red-500/30 bg-red-500/10'}"
            >
              {#if testResult.ok}
                <CheckCircle2 class="mt-0.5 size-4 shrink-0 text-green-400" />
                <p class="font-body text-sm text-green-300">
                  {testResult.message}
                </p>
              {:else}
                <AlertTriangle class="mt-0.5 size-4 shrink-0 text-red-400" />
                <p class="font-body text-sm text-red-300">
                  {testResult.message}
                </p>
              {/if}
            </div>
          {/if}

          <div class="flex flex-wrap gap-3">
            <button
              onclick={handleTestConnection}
              disabled={testing || saving}
              class="inline-flex cursor-pointer items-center gap-2 rounded-md border border-plumage px-4 py-2 font-body text-sm text-crown-ash transition-colors hover:border-talon-gold hover:text-talon-gold disabled:cursor-not-allowed disabled:opacity-50"
            >
              {#if testing}
                <Loader2 class="size-4 animate-spin" />
              {:else}
                <Plug class="size-4" />
              {/if}
              {translate("integrations.testConnection", $locale)}
            </button>
            <button
              onclick={handleAddServer}
              disabled={saving || testing}
              class="inline-flex cursor-pointer items-center gap-2 rounded-md bg-talon-gold px-4 py-2 font-body text-sm font-medium text-obsidian transition-all hover:bg-talon-gold-bright disabled:cursor-not-allowed disabled:opacity-50"
            >
              {#if saving}
                <Loader2 class="size-4 animate-spin" />
              {:else}
                <Plus class="size-4" />
              {/if}
              {translate("integrations.registerServer", $locale)}
            </button>
          </div>
        </div>
      </section>
    {/if}

    <div class="space-y-3">
      {#each servers as server (server.id)}
        <article
          class="rounded-lg border border-plumage bg-obsidian-light/60 transition-colors hover:border-talon-gold/40"
        >
          <div class="flex flex-wrap items-start justify-between gap-3 p-4">
            <button
              type="button"
              onclick={() => toggleExpanded(server.id)}
              class="flex min-w-0 flex-1 cursor-pointer items-start gap-3 text-left"
            >
              {#if isExpanded(server.id)}
                <ChevronDown class="mt-1 size-4 shrink-0 text-talon-gold" />
              {:else}
                <ChevronRight class="mt-1 size-4 shrink-0 text-crown-ash" />
              {/if}
              <div class="min-w-0">
                <div class="flex flex-wrap items-center gap-2">
                  <p class="font-heading text-base font-semibold text-cream">
                    {server.name}
                  </p>
                  <span
                    class="rounded-full border border-plumage px-2 py-0.5 font-mono text-[10px] tracking-wider text-crown-ash uppercase"
                  >
                    {kindLabel(server.kind)}
                  </span>
                </div>
                <p
                  class="mt-1 font-mono text-[10px] tracking-wider text-crown-ash-dark uppercase"
                >
                  {server.toolCount} tool{server.toolCount === 1 ? "" : "s"}
                </p>
                {#if server.lastError}
                  <p class="mt-2 font-body text-xs text-red-400">
                    {server.lastError}
                  </p>
                {/if}
              </div>
            </button>

            <div class="flex shrink-0 flex-wrap items-center gap-2">
              <McpStatusBadge status={server.status} />
              {#if server.status === "auth_required"}
                <button
                  onclick={() => handleConnect(server.id)}
                  disabled={connectingId === server.id}
                  class="inline-flex cursor-pointer items-center gap-1.5 rounded-md border border-talon-gold/50 bg-talon-gold/10 px-3 py-1.5 font-body text-xs text-talon-gold transition-colors hover:bg-talon-gold/20 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  {#if connectingId === server.id}
                    <Loader2 class="size-3.5 animate-spin" />
                  {:else}
                    <Link2 class="size-3.5" />
                  {/if}
                  {translate("integrations.connect", $locale)}
                </button>
              {/if}
            </div>
          </div>

          {#if isExpanded(server.id)}
            <div class="border-t border-plumage/60 px-4 py-4">
              <div class="mb-3 flex items-center gap-2">
                <Wrench class="size-4 text-talon-gold" />
                <p class="font-body text-sm font-medium text-cream">
                  {translate("integrations.tools", $locale)}
                </p>
              </div>

              {#if server.tools.length === 0}
                <p class="font-body text-sm text-crown-ash">
                  {translate("integrations.noToolsYet", $locale)}
                  {#if server.status === "auth_required"}
                    {translate("integrations.connectToDiscover", $locale)}
                  {/if}
                </p>
              {:else}
                <ul class="space-y-2">
                  {#each server.tools as tool (tool.name)}
                    <li
                      class="flex flex-wrap items-start justify-between gap-3 rounded-md border border-plumage/60 bg-obsidian/50 px-3 py-2"
                    >
                      <div class="min-w-0">
                        <p class="font-mono text-xs text-talon-gold">
                          {tool.name}
                        </p>
                        <p class="mt-0.5 font-body text-xs text-crown-ash">
                          {tool.description}
                        </p>
                      </div>
                      <button
                        type="button"
                        onclick={() => handleBindToAgent(server, tool.name)}
                        class="cursor-pointer rounded-md border border-plumage px-2.5 py-1 font-body text-[11px] text-crown-ash transition-colors hover:border-talon-gold hover:text-talon-gold"
                      >
                        {translate("integrations.bindToAgent", $locale)}
                      </button>
                    </li>
                  {/each}
                </ul>
              {/if}
            </div>
          {/if}
        </article>
      {:else}
        <div
          class="rounded-lg border border-dashed border-plumage px-6 py-12 text-center"
        >
          <Plug class="mx-auto mb-3 size-10 text-talon-gold" />
          <p class="font-body text-sm text-crown-ash">
            {translate("integrations.noServers", $locale)}
          </p>
        </div>
      {/each}
    </div>
  {/if}
</div>
