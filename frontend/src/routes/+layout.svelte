<script lang="ts">
	import { Menu, X, Sun, Moon, LayoutDashboard, Eye, Settings } from 'lucide-svelte';
	import TenantSelector from '$lib/components/TenantSelector.svelte';
	import FeedbackBadge from '$lib/components/FeedbackBadge.svelte';
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

	const sections = [
		{ label: 'Tasks', href: '/', icon: LayoutDashboard },
		{ label: 'Oversee', href: '/oversee', icon: Eye },
		{ label: 'Configure', href: '/configure', icon: Settings },
	];

	function isActive(path: string) {
		return page.url.pathname === path || (path === '/' && page.url.pathname === '/tasks');
	}
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

<div class="flex min-h-screen bg-[#121318]">
	<!-- Left gold stripe -->
	<div class="fixed inset-y-0 left-0 z-50 w-[5px] bg-gradient-to-b from-[#E0A512] via-[#C8920F] to-[#C8920F]/30"></div>

	<!-- Hidden lateral nav -->
	<div class="fixed inset-y-0 left-[5px] z-40 flex">
		<div class="h-full overflow-hidden transition-all duration-300 {navOpen ? 'w-64' : 'w-0'}">
			<nav class="h-full w-64 border-r border-[#2A2D3A] bg-[#121318] px-4 py-6">
				<div class="mb-8 flex items-center justify-between">
					<span class="font-semibold text-xl text-[#C8920F]" style="font-family: 'Bodoni Moda', serif">
						Harpia
					</span>
					<button onclick={() => (navOpen = false)}
						class="rounded-md p-1 text-[#9DA1AB] hover:text-[#F5F2EB] transition-colors cursor-pointer">
						<X class="size-5" />
					</button>
				</div>

				<div class="space-y-1">
					{#each sections as section}
						<a href={section.href}
							onclick={() => (navOpen = false)}
							class="flex items-center gap-3 rounded-md px-3 py-2.5 text-[#9DA1AB] transition-all hover:bg-[#1A1B24] hover:text-[#F5F2EB] cursor-pointer {isActive(section.href) ? 'bg-[#1A1B24] text-[#C8920F]' : ''}">
							<section.icon class="size-4" />
							<span class="text-sm font-medium" style="font-family: 'DM Sans', sans-serif">{section.label}</span>
							{#if section.label === 'Oversee'}
								<FeedbackBadge />
							{/if}
						</a>
					{/each}
				</div>

				<div class="mt-8 border-t border-[#2A2D3A] pt-6">
					<p class="text-[10px] uppercase tracking-widest text-[#6B7080]" style="font-family: 'JetBrains Mono', monospace">Role</p>
					<p class="mt-1 text-sm text-[#F5F2EB]" style="font-family: 'DM Sans', sans-serif">
						{data?.user?.role ?? 'Leader'}
					</p>
				</div>
			</nav>
		</div>
	</div>

	<!-- Main content -->
	<div class="flex flex-1 flex-col transition-all duration-300 {navOpen ? 'ml-[calc(5px+16rem)]' : 'ml-[5px]'}">
		{#if data?.user}
			<header class="sticky top-0 z-30 border-b border-[#2A2D3A]/30 bg-[#121318]/80 backdrop-blur">
				<div class="flex h-14 items-center justify-between px-4 lg:px-6">
					<div class="flex items-center gap-3">
						<button onclick={() => (navOpen = !navOpen)}
							class="rounded-md p-1.5 text-[#9DA1AB] hover:text-[#F5F2EB] transition-colors cursor-pointer"
							aria-label="Toggle navigation">
							<Menu class="size-5" />
						</button>
						<span class="text-lg font-bold tracking-tight text-[#F5F2EB]" style="font-family: 'Bodoni Moda', serif">
							<span class="text-[#C8920F]">Harp</span>ia
						</span>
					</div>

					<div class="flex items-center gap-4">
						<TenantSelector />
						<span class="text-sm text-[#9DA1AB]" style="font-family: 'DM Sans', sans-serif">{data.user.name}</span>
						<button onclick={logout}
							class="cursor-pointer rounded-md border border-[#2A2D3A] px-3 py-1 text-xs text-[#9DA1AB] transition-colors hover:border-[#C8920F] hover:text-[#C8920F]"
							style="font-family: 'DM Sans', sans-serif">
							Log out
						</button>
					</div>
				</div>
			</header>
		{/if}

		<main class="flex-1">
			{@render children()}
		</main>
	</div>
</div>
