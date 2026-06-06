<script lang="ts">
  import { ChevronDown } from "lucide-svelte";
  import { getSession, setTenant } from "$lib/auth";
  import type { Tenant } from "$lib/auth";

  let tenants = $state<Tenant[]>([]);
  let selectedTenant = $state<Tenant | null>(null);
  let open = $state(false);

  $effect(() => {
    // TODO: Replace with ConnectRPC IdentityService.ListTenants call
    tenants = [
      { id: "1", name: "Acme Corp" },
      { id: "2", name: "Globex Inc" },
      { id: "3", name: "Initech" },
    ];

    const session = getSession();
    if (session?.tenant) {
      selectedTenant = session.tenant;
    }
  });

  function select(tenant: Tenant) {
    selectedTenant = tenant;
    setTenant(tenant);
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
  <button
    onclick={() => (open = !open)}
    class="flex items-center gap-2 rounded-lg border border-[#2A2D3A] px-3 py-1.5 text-sm transition-colors hover:border-[#C8920F]"
    style="font-family: 'DM Sans', sans-serif;"
  >
    <span class="text-[#C8920F]">
      {selectedTenant?.name ?? "Select tenant"}
    </span>
    <ChevronDown
      class="size-3.5 text-[#6B7080] transition-transform {open
        ? 'rotate-180'
        : ''}"
    />
  </button>

  {#if open}
    <div
      class="absolute top-full right-0 z-50 mt-1 min-w-[180px] rounded-lg border border-[#2A2D3A] bg-[#1A1B24] py-1 shadow-lg"
    >
      {#each tenants as tenant (tenant.id)}
        <button
          onclick={() => select(tenant)}
          class="w-full px-4 py-2 text-left text-sm text-[#9DA1AB] transition-colors hover:bg-[#2A2D3A] hover:text-[#F5F2EB]"
          style="font-family: 'DM Sans', sans-serif;"
          aria-current={selectedTenant?.id === tenant.id ? "true" : undefined}
        >
          {tenant.name}
          {#if selectedTenant?.id === tenant.id}
            <span class="ml-2 text-[#C8920F]">&#10003;</span>
          {/if}
        </button>
      {/each}
    </div>
  {/if}
</div>
