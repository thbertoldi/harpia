<script lang="ts">
  import { fade } from "svelte/transition";
  import type { ChatMessage } from "$lib/chat/types";
  import { locale, translate } from "$lib/i18n";
  import { saveCelebration, chatEnter } from "$lib/motion/transitions";

  // chatEnter takes no params; wrap it to inject a per-chip stagger delay
  // while preserving its reduced-motion handling.
  function chatEnterDelayed(node: HTMLElement, params: { delay?: number }) {
    return { ...chatEnter(node), delay: params.delay ?? 0 };
  }
  import { planClient } from "$lib/rpc";
  import type { PlanConfiguration } from "$lib/gen/harpia/plans/v1/plans_pb";
  import ScheduleDialog from "$lib/components/canvas/ScheduleDialog.svelte";

  interface LandingAction {
    id: string;
    label: string;
  }

  interface Props {
    message: ChatMessage;
    configurationId: string;
    tenantId: string;
  }
  let { message, configurationId, tenantId }: Props = $props();

  const actions = $derived.by<LandingAction[]>(() => {
    try {
      const parsed = JSON.parse(message.payloadJson) as {
        actions?: LandingAction[];
      };
      return Array.isArray(parsed.actions) ? parsed.actions : [];
    } catch {
      return [];
    }
  });

  let busyAction = $state<string | null>(null);
  let dismissed = $state(false);
  let runError = $state(false);

  // The schedule action reuses the M5 ScheduleDialog, which needs the full
  // configuration. Loaded lazily when "schedule" is clicked.
  let scheduleOpen = $state(false);
  let configuration = $state<PlanConfiguration | null>(null);

  async function onAction(action: LandingAction) {
    if (busyAction) return;
    if (action.id === "run-now") {
      busyAction = action.id;
      runError = false;
      try {
        await planClient.createPlanExecution({
          tenantId,
          planConfigurationId: configurationId,
        });
        // The RUN_STARTED system event flows back through the thread watch;
        // the card stays so Ana can scroll to the inline DAG.
      } catch {
        runError = true;
      } finally {
        busyAction = null;
      }
      return;
    }
    if (action.id === "schedule") {
      busyAction = action.id;
      try {
        const res = await planClient.getPlanConfiguration({
          tenantId,
          planConfigurationId: configurationId,
        });
        if (res.planConfiguration) {
          configuration = res.planConfiguration;
          scheduleOpen = true;
        }
      } catch {
        runError = true;
      } finally {
        busyAction = null;
      }
      return;
    }
    if (action.id === "walk-away") {
      dismissed = true;
    }
  }

  function isPrimary(action: LandingAction): boolean {
    return action.id === "run-now";
  }
</script>

{#if !dismissed}
  <div
    id={`m-${message.id}`}
    in:saveCelebration
    class="flex flex-col items-center gap-4 rounded-lg border border-talon-gold/40 bg-surface-elevated px-6 py-7 text-center"
  >
    <!-- Checkmark that stroke-draws on mount -->
    <svg
      class="size-12"
      viewBox="0 0 52 52"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      aria-hidden="true"
    >
      <circle
        cx="26"
        cy="26"
        r="24"
        stroke="var(--token-primary, #c8920f)"
        stroke-width="2"
        opacity="0.4"
      />
      <path
        class="landing-check"
        d="M15 27 L23 35 L38 18"
        stroke="var(--token-primary, #c8920f)"
        stroke-width="3"
        stroke-linecap="round"
        stroke-linejoin="round"
      />
    </svg>

    {#if message.text}
      <p class="max-w-md font-body text-[14px] leading-relaxed text-cream">
        {translate("assistant.prompt.landing", $locale)}
      </p>
    {/if}

    <div class="mt-1 flex flex-wrap items-center justify-center gap-2.5">
      {#each actions as action, i (action.id)}
        <button
          type="button"
          in:chatEnterDelayed={{ delay: 120 + i * 90 }}
          disabled={busyAction !== null}
          onclick={() => onAction(action)}
          class={isPrimary(action)
            ? "cursor-pointer rounded-md bg-primary px-4 py-2 text-[13px] font-semibold text-on-primary shadow-[0_1px_0_rgba(255,255,255,0.15)_inset] hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-50"
            : "cursor-pointer rounded-md border border-plumage bg-transparent px-4 py-2 text-[13px] font-medium text-crown-ash hover:border-talon-gold hover:text-cream disabled:cursor-not-allowed disabled:opacity-50"}
        >
          {action.label}
        </button>
      {/each}
    </div>

    {#if runError}
      <p transition:fade class="text-[11px] text-red-400">
        {translate("thread.runError", $locale)}
      </p>
    {/if}
  </div>
{:else}
  <div
    id={`m-${message.id}`}
    class="rounded-md border border-plumage/60 bg-surface px-3 py-2 text-[11px] text-crown-ash"
  >
    {translate("assistant.landing.savedNote", $locale)}
  </div>
{/if}

{#if configuration}
  <ScheduleDialog
    open={scheduleOpen}
    {configuration}
    onClose={() => (scheduleOpen = false)}
    onSaved={(next) => (configuration = next)}
  />
{/if}

<style>
  .landing-check {
    stroke-dasharray: 48;
    stroke-dashoffset: 48;
    animation: landing-draw 600ms cubic-bezier(0.65, 0, 0.35, 1) 200ms forwards;
  }
  @keyframes landing-draw {
    to {
      stroke-dashoffset: 0;
    }
  }
  @media (prefers-reduced-motion: reduce) {
    .landing-check {
      animation: none;
      stroke-dashoffset: 0;
    }
  }
</style>
