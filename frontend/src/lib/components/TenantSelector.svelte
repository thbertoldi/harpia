<script lang="ts">
  import { ChevronDown } from "lucide-svelte";
  import {
    DEV_TENANT,
    getSession,
    logout,
    selectDefaultTenant,
    setTenant,
  } from "$lib/auth";
  import type { Tenant } from "$lib/auth";
  import { identityClient } from "$lib/rpc";
  import { initTheme } from "$lib/themes";
  import { locale, translate } from "$lib/i18n";
  import { chipFlash } from "$lib/motion/transitions";

  let tenants = $state<Tenant[]>([]);
  let selectedTenant = $state<Tenant | null>(null);
  let open = $state(false);
  let loaded = $state(false);
  let noTenantAccess = $state(false);

  $effect(() => {
    if (loaded) return;
    loaded = true;

    const session = getSession();
    if (session?.tenant) {
      selectedTenant = session.tenant;
      tenants = [session.tenant];
      return;
    }

    if (session?.tokens.access_token === "dev-token") {
      selectedTenant = DEV_TENANT;
      tenants = [DEV_TENANT];
      setTenant(DEV_TENANT);
      initTheme({ tenantThemeKey: DEV_TENANT.themeKey ?? null });
    } else {
      void loadTenants();
    }
  });

  async function loadTenants() {
    try {
      const response = await identityClient.listTenants({});
      tenants = response.tenants.map((tenant) => ({
        id: tenant.id,
        name: tenant.name,
        themeKey: tenant.themeKey || undefined,
      }));
      const defaultTenant = selectDefaultTenant(tenants);
      if (defaultTenant) {
        selectedTenant = defaultTenant;
        setTenant(defaultTenant);
        initTheme({ tenantThemeKey: defaultTenant.themeKey ?? null });
      } else if (tenants.length === 0) {
        noTenantAccess = true;
      }
    } catch {
      tenants = [];
      noTenantAccess = true;
    }
  }

  function select(tenant: Tenant) {
    selectedTenant = tenant;
    setTenant(tenant);
    initTheme({ tenantThemeKey: tenant.themeKey ?? null });
    open = false;
  }

  function handleClickOutside(e: MouseEvent) {
    const target = e.target as HTMLElement;
    if (!target.closest(".tenant-selector")) {
      open = false;
    }
  }

  $effect(() => {
    if (open) {
      document.addEventListener("click", handleClickOutside);
    }
    return () => {
      document.removeEventListener("click", handleClickOutside);
    };
  });
</script>

<div class="tenant-selector relative">
  {#if noTenantAccess}
    <div
      class="max-w-xs rounded-lg border border-border bg-surface-pop px-3 py-2 text-left"
    >
      <p class="font-body text-sm text-text">
        {translate("tenant.noAccess.title", $locale)}
      </p>
      <p class="mt-1 text-xs text-text-muted">
        {translate("tenant.noAccess.description", $locale)}
      </p>
      <button
        onclick={logout}
        in:chipFlash
        class="mt-2 text-xs text-primary transition-colors hover:text-primary-bright"
      >
        {translate("tenant.noAccess.signOut", $locale)}
      </button>
    </div>
  {:else}
    <button
      onclick={() => (open = !open)}
      class="flex items-center gap-2 rounded-lg border border-border px-3 py-1.5 font-body text-sm transition-colors hover:bg-surface-hover"
    >
      <span class="text-primary">
        {selectedTenant?.name ?? translate("tenant.select", $locale)}
      </span>
      <ChevronDown
        class="size-3.5 text-text-muted-dark transition-transform {open
          ? 'rotate-180'
          : ''}"
      />
    </button>

    {#if open}
      <div
        class="absolute top-full right-0 z-50 mt-1 min-w-[180px] rounded-lg border border-border bg-surface-pop py-1 shadow-lg"
      >
        {#each tenants as tenant (tenant.id)}
          <button
            onclick={() => select(tenant)}
            class="w-full px-4 py-2 text-left font-body text-sm text-text-muted transition-colors hover:bg-surface-hover hover:text-text"
            aria-current={selectedTenant?.id === tenant.id ? "true" : undefined}
          >
            {tenant.name}
            {#if selectedTenant?.id === tenant.id}
              <span class="ml-2 text-primary">&#10003;</span>
            {/if}
          </button>
        {/each}
      </div>
    {/if}
  {/if}
</div>
