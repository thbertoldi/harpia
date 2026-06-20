<script lang="ts">
  import MilestoneStub from "$lib/components/MilestoneStub.svelte";
  import { locale, translate } from "$lib/i18n";
</script>

<svelte:head>
  <title>{translate("nav.discover", $locale)} · Harpia</title>
</svelte:head>

<MilestoneStub title={translate("nav.discover", $locale)} milestone="M5" />
