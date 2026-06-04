<script lang="ts">
	import '../app.css';
	import TenantSelector from '$lib/components/TenantSelector.svelte';
	import { logout } from '$lib/auth';

	let { data, children } = $props();
</script>

<svelte:head>
	<script>
		(() => {
			const theme = localStorage.getItem('mode-watcher-theme') ?? 'system';
			const dark =
				theme === 'dark' ||
				(theme === 'system' &&
					window.matchMedia('(prefers-color-scheme: dark)').matches);
			document.documentElement.classList.toggle('dark', dark);
		})();
	</script>
</svelte:head>

{#if data?.user}
	<header
		class="sticky top-0 z-40 border-b border-[#2A2D3A]/30 bg-[#121318]/80 backdrop-blur"
	>
		<div class="flex h-14 items-center justify-between px-6">
			<span
				class="text-lg font-bold tracking-tight text-[#C8920F]"
				style="font-family: 'Bodoni Moda', serif;"
			>
				Harpia
			</span>

			<div class="flex items-center gap-4">
				<TenantSelector />

				<span class="text-sm text-[#9DA1AB]" style="font-family: 'DM Sans', sans-serif;">
					{data.user.name}
				</span>

				<button
					onclick={logout}
					class="cursor-pointer rounded-lg border border-[#2A2D3A] px-3 py-1 text-xs text-[#9DA1AB] transition-colors hover:border-[#C8920F] hover:text-[#C8920F]"
					style="font-family: 'DM Sans', sans-serif;"
				>
					Log out
				</button>
			</div>
		</div>
	</header>
{/if}

{@render children()}
