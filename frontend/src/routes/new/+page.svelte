<script lang="ts">
  import { onMount } from "svelte";
  import { goto } from "$app/navigation";
  import { resolve } from "$app/paths";
  import { getTenant } from "$lib/auth";
  import { locale, translate } from "$lib/i18n";
  import { planClient, threadClient } from "$lib/rpc";
  import { PlanConfigurationStatus } from "$lib/gen/harpia/plans/v1/plans_pb";
  import HarpyHeading from "$lib/components/ui/HarpyHeading.svelte";
  import TemplatePickerCard from "$lib/components/thread/TemplatePickerCard.svelte";
  import {
    genericInputInitialValues,
    genericParameterValuesJson,
  } from "$lib/plans/template-inputs";

  let { data } = $props();
  const tenantId = $derived(getTenant()?.id ?? "");

  let creating = $state(false);
  let createError = $state<string | null>(null);
  let composerText = $state("");

  const matchedTemplate = $derived.by(() => {
    const q = composerText.trim().toLowerCase();
    if (q.length < 2) return null;
    const hit = data.templates.find(
      (t) =>
        t.name.toLowerCase().includes(q) ||
        (t.description ?? "").toLowerCase().includes(q),
    );
    return hit ?? null;
  });

  async function startFromPrompt() {
    if (creating || !tenantId) return;
    const text = composerText.trim();
    if (text.length === 0) return;
    creating = true;
    createError = null;
    try {
      const threadResponse = await threadClient.createThread({
        tenantId,
        title: text.slice(0, 60),
        initialMessageText: text,
      });
      const threadId = threadResponse.thread?.id;
      if (!threadId) throw new Error("createThread returned no id");
      await goto(resolve(`/chat/${encodeURIComponent(threadId)}`));
    } catch (e) {
      createError = e instanceof Error ? e.message : "Failed to start";
    } finally {
      creating = false;
    }
  }

  async function pickTemplate(templateId: string) {
    if (creating || !tenantId) return;
    creating = true;
    createError = null;
    try {
      const template = data.templates.find(
        (candidate) => candidate.id === templateId,
      );
      const parameterValuesJson = genericParameterValuesJson(
        genericInputInitialValues(template?.inputParameters ?? [], {}),
      );
      const threadResponse = await threadClient.createThread({
        tenantId,
        title: matchedTemplate?.name ?? "Untitled chat",
        initialMessageText: composerText.trim(),
      });
      const threadId = threadResponse.thread?.id;
      if (!threadId) throw new Error("createThread returned no id");

      const response = await planClient.createPlanConfiguration({
        tenantId,
        workspaceId: "",
        planTemplateId: templateId,
        status: PlanConfigurationStatus.DRAFT,
        threadId,
        parameterValuesJson,
      });
      const configId = response.planConfiguration?.id;
      if (!configId) throw new Error("createPlanConfiguration returned no id");
      // The standalone configuration surface was removed (stabilize-user-journey);
      // plans are configured in-conversation, so land on the freshly created thread.
      await goto(resolve(`/chat/${encodeURIComponent(threadId)}`));
    } catch (e) {
      createError = e instanceof Error ? e.message : "Failed to create plan";
    } finally {
      creating = false;
    }
  }

  onMount(() => {
    if (data.autoTemplateId) void pickTemplate(data.autoTemplateId);
  });
</script>

<svelte:head
  ><title>{translate("nav.newPlan", $locale)} · Harpia</title></svelte:head
>

<div class="mx-auto max-w-3xl px-4 py-6">
  <HarpyHeading tag="h1" class="text-2xl text-cream">
    {translate("new.greeting", $locale)}
  </HarpyHeading>
  <p class="mt-1 font-body text-[13px] text-crown-ash">
    {translate("new.subgreeting", $locale)}
  </p>

  <div class="mt-4">
    <input
      type="text"
      bind:value={composerText}
      onkeydown={(e) => {
        if (e.key !== "Enter") return;
        if (matchedTemplate) void pickTemplate(matchedTemplate.id);
        else void startFromPrompt();
      }}
      placeholder={translate("new.composerPlaceholder", $locale)}
      class="w-full rounded-md border border-plumage bg-obsidian-light px-3 py-2 text-[13px] text-cream"
    />
    {#if composerText.trim().length >= 2 && !matchedTemplate}
      <p class="mt-1 text-[11px] text-crown-ash-dark">
        {translate("new.noMatch", $locale, { query: composerText })}
      </p>
    {/if}
  </div>

  <div class="mt-6">
    <p
      class="mb-2 font-mono text-[10px] tracking-widest text-crown-ash-dark uppercase"
    >
      {translate("new.galleryHeading", $locale)}
    </p>
    <TemplatePickerCard
      templates={data.templates}
      onPick={pickTemplate}
      disabled={creating}
    />
  </div>

  {#if createError}
    <p class="mt-3 text-[11px] text-red-400">{createError}</p>
  {/if}
</div>
