<script lang="ts">
  import { locale, translate } from "$lib/i18n";
  import CanvasDrawer from "./CanvasDrawer.svelte";
  import PlanPoliciesForm from "$lib/components/PlanPoliciesForm.svelte";
  import { planClient } from "$lib/rpc";
  import { toUserMessage } from "$lib/connect-errors";
  import {
    DEFAULT_BEHAVIOR_POLICIES,
    policiesFromProto,
    policiesToProto,
    type BehaviorPoliciesFormValues,
  } from "$lib/plans/behavior-policies";
  import type { PlanConfiguration } from "$lib/gen/harpia/plans/v1/plans_pb";

  interface Props {
    open: boolean;
    onClose: () => void;
    configurationId: string;
    tenantId: string;
  }
  let { open, onClose, configurationId, tenantId }: Props = $props();

  let values = $state<BehaviorPoliciesFormValues>({
    ...DEFAULT_BEHAVIOR_POLICIES,
  });
  let configuration = $state<PlanConfiguration | null>(null);
  let loading = $state(false);
  let loadError = $state<string | null>(null);
  let saving = $state(false);
  let saveError = $state<string | null>(null);
  let saveNotice = $state<string | null>(null);

  $effect(() => {
    if (!open) return;
    loading = true;
    loadError = null;
    void (async () => {
      try {
        const response = await planClient.getPlanConfiguration({
          tenantId,
          planConfigurationId: configurationId,
        });
        configuration = response.planConfiguration ?? null;
        values = policiesFromProto(configuration?.behaviorPolicies);
      } catch (e) {
        loadError = toUserMessage(e);
      } finally {
        loading = false;
      }
    })();
  });

  async function handleSave() {
    if (!configuration) return;
    saving = true;
    saveError = null;
    saveNotice = null;
    try {
      const response = await planClient.updatePlanConfiguration({
        tenantId,
        planConfigurationId: configurationId,
        status: configuration.status,
        seedArtifacts: configuration.seedArtifacts,
        slotBindings: configuration.slotBindings,
        overseerBindings: configuration.overseerBindings,
        behaviorPolicies: policiesToProto(values),
        schedule: configuration.schedule,
      });
      configuration = response.planConfiguration ?? configuration;
      saveNotice = translate("canvas.settings.saved", $locale);
    } catch (e) {
      saveError = toUserMessage(e);
    } finally {
      saving = false;
    }
  }
</script>

<CanvasDrawer
  {open}
  {onClose}
  title={translate("canvas.settings.title", $locale)}
>
  {#if loading}
    <p class="text-[12px] text-crown-ash-dark">
      {translate("canvas.settings.loading", $locale)}
    </p>
  {:else if loadError}
    <p class="text-[12px] text-red-400">{loadError}</p>
  {:else}
    <PlanPoliciesForm
      bind:values
      {saving}
      {saveError}
      {saveNotice}
      onSave={handleSave}
    />
  {/if}
</CanvasDrawer>
