<script lang="ts">
  import MilestoneStub from "$lib/components/MilestoneStub.svelte";
  import { locale, translate } from "$lib/i18n";
</script>

<svelte:head>
  <title>{translate("nav.newPlan", $locale)} · Harpia</title>
</svelte:head>

<MilestoneStub title={translate("nav.newPlan", $locale)} milestone="M5" />
