<script lang="ts">
  import { Loader2 } from "lucide-svelte";
  import { locale, translate } from "$lib/i18n";
  import { requireTenantId } from "$lib/auth";
  import {
    buildPayloadJson,
    parseSchemaFields,
    respondToElicitation,
    type ElicitationFormField,
    type ElicitationRequest,
  } from "$lib/plans/elicitations";
  import { toUserMessage } from "$lib/connect-errors";

  interface Props {
    elicitation: ElicitationRequest;
    disabled?: boolean;
    onAnswered?: (updated: ElicitationRequest) => void;
  }
  let { elicitation, disabled = false, onAnswered }: Props = $props();

  const schemaFields = $derived<ElicitationFormField[]>(
    parseSchemaFields(elicitation.schemaJson),
  );

  let fieldValues = $state<Record<string, string>>({});
  let responseText = $state("");
  let rawJson = $state("");
  let fieldError = $state<string | null>(null);
  let submitting = $state(false);
  let submitted = $state(false);
  let submitError = $state<string | null>(null);

  function validateRequiredFields(): boolean {
    for (const field of schemaFields) {
      if (field.required && !fieldValues[field.name]) {
        fieldError = translate("elicitations.form.required", $locale);
        return false;
      }
    }
    fieldError = null;
    return true;
  }

  async function submit(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    if (disabled || submitting) {
      return;
    }
    if (!validateRequiredFields()) {
      return;
    }

    let payloadJson = "";
    if (schemaFields.length > 0) {
      payloadJson = buildPayloadJson(fieldValues);
    } else if (rawJson.trim() !== "") {
      try {
        JSON.parse(rawJson);
        payloadJson = rawJson.trim();
      } catch {
        submitError = translate("elicitations.form.error", $locale, {
          error: "invalid JSON",
        });
        return;
      }
    }

    submitting = true;
    submitError = null;

    try {
      const tenantId = requireTenantId();
      const updated = await respondToElicitation(tenantId, elicitation.id, {
        payloadJson,
        responseText,
      });
      submitted = true;
      responseText = "";
      rawJson = "";
      fieldValues = {};
      onAnswered?.(updated);
    } catch (error) {
      submitError = translate("elicitations.form.error", $locale, {
        error: toUserMessage(error),
      });
    } finally {
      submitting = false;
    }
  }
</script>

{#if disabled}
  <p class="font-body text-sm text-crown-ash">
    {translate("elicitations.form.disabled", $locale)}
  </p>
{:else}
  <form class="space-y-4" onsubmit={submit}>
    {#each schemaFields as field (field.name)}
      <div class="space-y-1">
        <label
          for={`field-${field.name}`}
          class="block font-body text-xs text-crown-ash"
        >
          {field.label}{field.required ? " *" : ""}
        </label>
        {#if field.type === "select"}
          <select
            id={`field-${field.name}`}
            bind:value={fieldValues[field.name]}
            class="w-full rounded-md border border-plumage bg-obsidian px-3 py-2 font-body text-sm text-cream focus:border-talon-gold focus:outline-none"
          >
            <option value="">—</option>
            {#each field.options ?? [] as option (option)}
              <option value={option}>{option}</option>
            {/each}
          </select>
        {:else if field.type === "textarea"}
          <textarea
            id={`field-${field.name}`}
            bind:value={fieldValues[field.name]}
            rows="3"
            class="w-full rounded-md border border-plumage bg-obsidian px-3 py-2 font-body text-sm text-cream focus:border-talon-gold focus:outline-none"
          ></textarea>
        {:else}
          <input
            id={`field-${field.name}`}
            type={field.type === "number" ? "number" : "text"}
            bind:value={fieldValues[field.name]}
            class="w-full rounded-md border border-plumage bg-obsidian px-3 py-2 font-body text-sm text-cream focus:border-talon-gold focus:outline-none"
          />
        {/if}
      </div>
    {/each}

    <div class="space-y-1">
      <label for="response-text" class="block font-body text-xs text-crown-ash">
        {translate("elicitations.form.responseText", $locale)}
      </label>
      <textarea
        id="response-text"
        bind:value={responseText}
        rows="3"
        placeholder={translate(
          "elicitations.form.responseTextPlaceholder",
          $locale,
        )}
        class="w-full rounded-md border border-plumage bg-obsidian px-3 py-2 font-body text-sm text-cream focus:border-talon-gold focus:outline-none"
      ></textarea>
    </div>

    {#if schemaFields.length === 0}
      <div class="space-y-1">
        <label
          for="response-json"
          class="block font-body text-xs text-crown-ash"
        >
          {translate("elicitations.form.jsonLabel", $locale)}
        </label>
        <textarea
          id="response-json"
          bind:value={rawJson}
          rows="3"
          placeholder={translate("elicitations.form.jsonPlaceholder", $locale)}
          class="w-full rounded-md border border-plumage bg-obsidian px-3 py-2 font-mono text-xs text-cream focus:border-talon-gold focus:outline-none"
        ></textarea>
      </div>
    {/if}

    {#if fieldError}
      <p class="font-body text-xs text-red-400">{fieldError}</p>
    {/if}
    {#if submitError}
      <p class="font-body text-xs text-red-400">{submitError}</p>
    {/if}
    {#if submitted}
      <p class="font-body text-xs text-green-400">
        {translate("elicitations.form.submitted", $locale)}
      </p>
    {/if}

    <button
      type="submit"
      disabled={submitting}
      class="inline-flex cursor-pointer items-center gap-2 rounded-md border border-talon-gold bg-talon-gold/10 px-4 py-2 font-body text-sm text-talon-gold transition-colors hover:bg-talon-gold/20 disabled:cursor-not-allowed disabled:opacity-60"
    >
      {#if submitting}
        <Loader2 class="size-4 animate-spin" />
        {translate("elicitations.form.submitting", $locale)}
      {:else}
        {translate("elicitations.form.submit", $locale)}
      {/if}
    </button>
  </form>
{/if}
