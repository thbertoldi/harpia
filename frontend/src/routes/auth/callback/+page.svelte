<script lang="ts">
  import { goto } from "$app/navigation";
  import { resolve } from "$app/paths";
  import { page } from "$app/stores";
  import { handleCallback } from "$lib/auth";
  import { locale, translate } from "$lib/i18n";

  let error = $state<string | null>(null);

  $effect(() => {
    const code = $page.url.searchParams.get("code");
    const state = $page.url.searchParams.get("state");

    if (!code) {
      error = translate("auth.callback.noCode", $locale);
      return;
    }

    const savedState = sessionStorage.getItem("oauth_state");
    if (state !== savedState) {
      error = translate("auth.callback.stateMismatch", $locale);
      return;
    }

    handleCallback(code)
      .then(() => goto(resolve("/")))
      .catch((e: Error) => {
        error = e.message || translate("auth.callback.failed", $locale);
      });
  });
</script>

<div class="flex min-h-screen bg-[#121318]">
  <div
    class="w-[5px] shrink-0 bg-gradient-to-b from-[#C8920F] via-[#E0A512] to-[#C8920F]"
  ></div>

  <div class="flex flex-1 flex-col items-center justify-center px-8">
    {#if error}
      <h1
        class="font-[Bodoni_Moda] text-4xl font-bold tracking-tight text-[#C8920F]"
      >
        {translate("auth.callback.failed", $locale)}
      </h1>
      <p class="mt-4 max-w-md text-center text-sm text-[#9DA1AB]">
        {error}
      </p>
      <a
        href={resolve("/login")}
        class="mt-10 inline-block rounded-lg bg-[#C8920F] px-6 py-2.5 text-sm font-medium text-[#121318] transition-colors hover:bg-[#E0A512]"
      >
        {translate("auth.callback.returnLogin", $locale)}
      </a>
    {:else}
      <div
        class="size-8 animate-spin rounded-full border-2 border-[#C8920F] border-t-transparent"
      ></div>
      <p class="mt-4 text-sm text-[#9DA1AB]">
        {translate("auth.callback.completing", $locale)}
      </p>
    {/if}
  </div>
</div>
