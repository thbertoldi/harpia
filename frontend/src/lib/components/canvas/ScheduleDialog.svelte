<script lang="ts">
  import { X } from "lucide-svelte";
  import { planClient } from "$lib/rpc";
  import { locale, translate } from "$lib/i18n";
  import type {
    PlanConfiguration,
    PlanSchedule,
  } from "$lib/gen/harpia/plans/v1/plans_pb";

  interface Props {
    open: boolean;
    configuration: PlanConfiguration;
    onClose: () => void;
    onSaved?: (next: PlanConfiguration) => void;
  }
  let { open, configuration, onClose, onSaved }: Props = $props();

  type Cadence = "manual" | "daily" | "weekly" | "monthly" | "custom";

  let cadence = $state<Cadence>("manual");
  let time = $state("09:00");
  let dayOfWeek = $state(1);
  let dayOfMonth = $state(1);
  let customCron = $state("");
  let error = $state<string | null>(null);
  let saving = $state(false);

  function cronFromInputs(): string {
    if (cadence === "manual") return "";
    if (cadence === "custom") return customCron.trim();
    const [hh, mm] = time.split(":").map((s) => parseInt(s, 10) || 0);
    if (cadence === "daily") return `${mm} ${hh} * * *`;
    if (cadence === "weekly") return `${mm} ${hh} * * ${dayOfWeek}`;
    if (cadence === "monthly") return `${mm} ${hh} ${dayOfMonth} * *`;
    return "";
  }

  async function save() {
    error = null;
    saving = true;
    try {
      const cron = cronFromInputs();
      const response = await planClient.updatePlanConfiguration({
        tenantId: configuration.tenantId,
        planConfigurationId: configuration.id,
        status: configuration.status,
        seedArtifacts: configuration.seedArtifacts,
        slotBindings: configuration.slotBindings,
        overseerBindings: configuration.overseerBindings,
        behaviorPolicies: configuration.behaviorPolicies,
        schedule: {
          cronExpression: cron,
          timezone: configuration.schedule?.timezone || "UTC",
        } satisfies PlanSchedule,
      });
      if (response.planConfiguration && onSaved)
        onSaved(response.planConfiguration);
      onClose();
    } catch (e) {
      error = e instanceof Error ? e.message : "Failed to save schedule";
    } finally {
      saving = false;
    }
  }
</script>

{#if open}
  <div
    class="fixed inset-0 z-50 flex items-center justify-center bg-obsidian/80 p-4"
  >
    <div
      class="w-full max-w-md rounded-lg border border-plumage bg-obsidian-light p-4"
    >
      <div class="mb-3 flex items-center justify-between">
        <h2 class="font-heading text-[14px] font-semibold text-cream">
          {translate("schedule.title", $locale)}
        </h2>
        <button
          type="button"
          onclick={onClose}
          aria-label={translate("schedule.close", $locale)}
          class="cursor-pointer text-crown-ash hover:text-cream"
        >
          <X class="size-4" />
        </button>
      </div>

      <div class="space-y-3">
        <div class="flex flex-wrap gap-2">
          {#each ["manual", "daily", "weekly", "monthly", "custom"] as c (c)}
            <button
              type="button"
              onclick={() => (cadence = c as Cadence)}
              class="cursor-pointer rounded-md border px-2 py-1 text-[11px] {cadence ===
              c
                ? 'border-talon-gold bg-talon-gold/10 text-talon-gold'
                : 'border-plumage text-crown-ash hover:border-talon-gold'}"
            >
              {translate(`schedule.cadence.${c}`, $locale)}
            </button>
          {/each}
        </div>

        {#if cadence === "daily" || cadence === "weekly" || cadence === "monthly"}
          <label class="block text-[11px] text-crown-ash">
            {translate("schedule.time", $locale)}
            <input
              type="time"
              bind:value={time}
              class="ml-2 rounded border border-plumage bg-obsidian px-2 py-1 text-[12px] text-cream"
            />
          </label>
        {/if}
        {#if cadence === "weekly"}
          <label class="block text-[11px] text-crown-ash">
            {translate("schedule.dayOfWeek", $locale)}
            <select
              bind:value={dayOfWeek}
              class="ml-2 rounded border border-plumage bg-obsidian px-2 py-1 text-[12px] text-cream"
            >
              <option value={0}>{translate("schedule.dow.0", $locale)}</option>
              <option value={1}>{translate("schedule.dow.1", $locale)}</option>
              <option value={2}>{translate("schedule.dow.2", $locale)}</option>
              <option value={3}>{translate("schedule.dow.3", $locale)}</option>
              <option value={4}>{translate("schedule.dow.4", $locale)}</option>
              <option value={5}>{translate("schedule.dow.5", $locale)}</option>
              <option value={6}>{translate("schedule.dow.6", $locale)}</option>
            </select>
          </label>
        {/if}
        {#if cadence === "monthly"}
          <label class="block text-[11px] text-crown-ash">
            {translate("schedule.dayOfMonth", $locale)}
            <input
              type="number"
              min="1"
              max="28"
              bind:value={dayOfMonth}
              class="ml-2 w-16 rounded border border-plumage bg-obsidian px-2 py-1 text-[12px] text-cream"
            />
          </label>
        {/if}
        {#if cadence === "custom"}
          <label class="block text-[11px] text-crown-ash">
            {translate("schedule.customCron", $locale)}
            <input
              type="text"
              bind:value={customCron}
              placeholder="0 9 * * *"
              class="mt-1 w-full rounded border border-plumage bg-obsidian px-2 py-1 font-mono text-[12px] text-cream"
            />
          </label>
        {/if}

        {#if error}
          <p class="text-[11px] text-red-400">{error}</p>
        {/if}
      </div>

      <div class="mt-4 flex justify-end gap-2">
        <button
          type="button"
          onclick={onClose}
          class="cursor-pointer rounded-md border border-plumage px-3 py-1.5 text-[12px] text-crown-ash hover:border-talon-gold hover:text-talon-gold"
        >
          {translate("schedule.cancel", $locale)}
        </button>
        <button
          type="button"
          disabled={saving}
          onclick={save}
          class="cursor-pointer rounded-md border border-talon-gold bg-talon-gold px-3 py-1.5 text-[12px] font-semibold text-obsidian hover:opacity-90 disabled:opacity-50"
        >
          {translate("schedule.save", $locale)}
        </button>
      </div>
    </div>
  </div>
{/if}
