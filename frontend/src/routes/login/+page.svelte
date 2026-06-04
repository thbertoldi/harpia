<script lang="ts">
	import { goto } from '$app/navigation';
	import { login, getSession } from '$lib/auth';
	import { Sun, Moon } from 'lucide-svelte';

	let email = $state('');
	let dark = $state(true);

	$effect(() => {
		if (getSession()) {
			goto('/');
		}
	});

	function toggleDark() {
		dark = !dark;
		document.documentElement.classList.toggle('dark', dark);
		localStorage.setItem('harpia-theme', dark ? 'dark' : 'light');
	}

	async function handleLogin() {
		await login(email ? { login_hint: email } : undefined);
	}
</script>

<div class="flex min-h-screen items-center justify-center bg-obsidian">
	<div class="fixed left-0 top-0 h-full w-[5px] bg-gradient-to-b from-talon-gold-bright via-talon-gold to-talon-gold/30"></div>

	<button
		onclick={toggleDark}
		class="fixed right-4 top-4 z-50 cursor-pointer rounded-md border border-plumage p-2 text-crown-ash transition-colors hover:border-talon-gold hover:text-cream"
		aria-label="Toggle dark mode"
	>
		{#if dark}
			<Sun class="size-4" />
		{:else}
			<Moon class="size-4" />
		{/if}
	</button>

	<div class="w-full max-w-md space-y-8 p-8">
		<div class="text-center">
			<h1 class="font-heading text-5xl font-black text-talon-gold tracking-tight" style="font-family: 'Bodoni Moda', serif">
				Harpia
			</h1>
			<p class="mt-3 font-body text-cream/70" style="font-family: 'DM Sans', sans-serif">
				AI operations for your business
			</p>
		</div>

		<div class="space-y-4">
			<button
				onclick={handleLogin}
				class="w-full cursor-pointer rounded-lg bg-talon-gold px-6 py-3 font-medium text-obsidian transition-all hover:bg-talon-gold-bright"
				style="font-family: 'DM Sans', sans-serif"
			>
				Sign in with Zitadel
			</button>

			<div class="relative">
				<div class="absolute inset-0 flex items-center">
					<div class="w-full border-t border-plumage"></div>
				</div>
				<div class="relative flex justify-center text-xs">
					<span class="bg-obsidian px-2 font-mono text-crown-ash uppercase tracking-wider" style="font-family: 'JetBrains Mono', monospace">
						or
					</span>
				</div>
			</div>

			<form onsubmit={(e) => { e.preventDefault(); handleLogin(); }} class="space-y-3">
				<input
					type="email"
					bind:value={email}
					placeholder="admin@harpia.local"
					class="w-full rounded-lg border border-plumage bg-obsidian-light px-4 py-3 font-body text-cream placeholder:text-crown-ash-dark focus:border-talon-gold focus:outline-none focus:ring-1 focus:ring-talon-gold transition-colors"
					style="font-family: 'DM Sans', sans-serif"
				/>
				<button
					type="submit"
					class="w-full cursor-pointer rounded-lg border border-plumage px-6 py-3 font-body text-sm text-crown-ash transition-colors hover:border-talon-gold hover:text-talon-gold"
					style="font-family: 'DM Sans', sans-serif"
				>
					Continue with email
				</button>
			</form>
		</div>
	</div>
</div>
