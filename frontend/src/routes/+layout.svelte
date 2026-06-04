<script lang="ts">
	import '../app.css';
	import { Menu, X, Sun, Moon, LayoutDashboard, Eye, Settings } from 'lucide-svelte';
	import GoldStripe from '$lib/components/ui/GoldStripe.svelte';
	import TenantSelector from '$lib/components/TenantSelector.svelte';
	import { logout } from '$lib/auth';
	import { page } from '$app/state';

	let { data, children } = $props();

	let navOpen = $state(false);
	let dark = $state(false);

	$effect(() => {
		const stored = localStorage.getItem('harpia-theme');
		const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
		const isDark = stored === 'dark' || (stored !== 'light' && prefersDark);
		dark = isDark;
		document.documentElement.classList.toggle('dark', isDark);
	});

	function toggleDark() {
		dark = !dark;
		document.documentElement.classList.toggle('dark', dark);
		localStorage.setItem('harpia-theme', dark ? 'dark' : 'light');
	}

	function toggleNav() {
		navOpen = !navOpen;
	}

	const sections = [
		{ label: 'Tasks', href: '/', icon: LayoutDashboard, description: 'Task dashboard' },
		{ label: 'Oversee', href: '/oversee', icon: Eye, description: 'Review agent output' },
		{ label: 'Configure', href: '/configure', icon: Settings, description: 'Admin panel' },
	];
</script>

<svelte:head>
	<script>
		(() => {
			const theme = localStorage.getItem('harpia-theme');
			const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
			const isDark = theme === 'dark' || (theme !== 'light' && prefersDark);
			document.documentElement.classList.toggle('dark', isDark);
		})();
	</script>
</svelte:head>

<div class="flex min-h-screen">
	<div class="fixed inset-y-0 z-40 flex">
		<GoldStripe />

		<!-- Hidden lateral nav -->
		<div class="h-full overflow-hidden transition-all duration-300 {navOpen ? 'w-64' : 'w-0'}">
			<nav class="h-full w-64 border-r border-plumage bg-obsidian px-4 py-6">
				<div class="mb-8 flex items-center justify-between">
					<span class="font-heading text-xl font-bold tracking-tight text-talon-gold">
						Harpia
					</span>
					<button
						onclick={toggleNav}
						class="rounded-md p-1 text-crown-ash hover:text-cream transition-colors"
					>
						<X class="size-5" />
					</button>
				</div>

				<div class="space-y-1">
					{#each sections as section}
						<a
							href={section.href}
							onclick={() => (navOpen = false)}
							class="flex items-center gap-3 rounded-md px-3 py-2.5 text-crown-ash transition-all hover:bg-obsidian-light hover:text-cream {page.url.pathname === section.href ? 'bg-obsidian-light text-talon-gold' : ''}"
						>
							<section.icon class="size-4" />
							<span class="font-body text-sm font-medium">{section.label}</span>
						</a>
					{/each}
				</div>

				<div class="mt-8 border-t border-plumage pt-6">
					<p class="font-mono text-[10px] uppercase tracking-widest text-crown-ash">
						Role
					</p>
					<p class="mt-1 font-body text-sm text-cream">
						{data?.user?.role ?? 'Leader'}
					</p>
				</div>
			</nav>
		</div>
	</div>

	<!-- Main content -->
	<div
		class="flex flex-1 flex-col {navOpen ? 'ml-[calc(5px+16rem)]' : 'ml-[5px]'} transition-all duration-300"
	>
		<!-- Header -->
		<header class="sticky top-0 z-30 border-b border-plumage/30 bg-obsidian/80 backdrop-blur">
			<div class="flex h-14 items-center justify-between px-4 lg:px-6">
				<div class="flex items-center gap-3">
					<button
						onclick={toggleNav}
						class="rounded-md p-1.5 text-crown-ash hover:text-cream transition-colors"
						aria-label="Toggle navigation"
					>
						<Menu class="size-5" />
					</button>
					<span class="font-heading text-lg font-bold tracking-tight text-cream">
						<span class="text-talon-gold">Harp</span>ia
					</span>
				</div>

				<div class="flex items-center gap-4">
					{#if data?.user}
						<TenantSelector />
						<span class="font-body text-sm text-crown-ash">{data.user.name}</span>
						<button
							onclick={logout}
							class="cursor-pointer rounded-md border border-plumage px-3 py-1 font-body text-xs text-crown-ash transition-colors hover:border-talon-gold hover:text-talon-gold"
						>
							Log out
						</button>
					{/if}

					<button
						onclick={toggleDark}
						class="rounded-md p-2 text-crown-ash hover:text-cream transition-colors"
						aria-label="Toggle dark mode"
					>
						{#if dark}
							<Sun class="size-4" />
						{:else}
							<Moon class="size-4" />
						{/if}
					</button>
				</div>
			</div>
		</header>

		<main class="flex-1">
			{@render children()}
		</main>
	</div>
</div>
