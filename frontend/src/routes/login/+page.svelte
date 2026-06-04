<script lang="ts">
	import { goto } from '$app/navigation';
	import { login, getSession } from '$lib/auth';

	let email = $state('');

	$effect(() => {
		if (getSession()) {
			goto('/');
		}
	});

	function handleLogin() {
		login({ login_hint: email || undefined });
	}

	function handleSubmit(e: Event) {
		e.preventDefault();
		handleLogin();
	}
</script>

<div class="flex min-h-screen bg-[#121318]">
	<div class="w-[5px] shrink-0 bg-gradient-to-b from-[#C8920F] via-[#E0A512] to-[#C8920F]"></div>

	<div class="flex flex-1 flex-col items-center justify-center px-8">
		<div class="w-full max-w-md">
			<h1
				class="font-[Bodoni_Moda] text-6xl font-bold tracking-tight text-[#C8920F]"
			>
				Harpia
			</h1>
			<p class="mt-2 text-lg text-[#9DA1AB]" style="font-family: 'DM Sans', sans-serif;">
				AI operations for your business
			</p>

			<div class="mt-12">
				<button
					onclick={() => login()}
					class="w-full cursor-pointer rounded-lg bg-[#C8920F] px-6 py-3 text-base font-medium text-[#121318] transition-colors hover:bg-[#E0A512]"
					style="font-family: 'DM Sans', sans-serif;"
				>
					Sign in with Zitadel
				</button>
			</div>

			<div class="mt-8 flex items-center gap-4">
				<div class="h-px flex-1 bg-[#2A2D3A]"></div>
				<span
					class="shrink-0 text-xs text-[#6B7080]"
					style="font-family: 'JetBrains Mono', monospace;"
				>
					or continue with
				</span>
				<div class="h-px flex-1 bg-[#2A2D3A]"></div>
			</div>

			<form onsubmit={handleSubmit} class="mt-6 space-y-4">
				<input
					type="email"
					placeholder="Email address"
					bind:value={email}
					class="w-full rounded-lg border border-[#2A2D3A] bg-[#1A1B24] px-4 py-2.5 text-sm text-[#F5F2EB] placeholder:text-[#6B7080] focus:border-[#C8920F] focus:outline-none"
					style="font-family: 'DM Sans', sans-serif;"
				/>
				<button
					type="submit"
					class="w-full cursor-pointer rounded-lg border border-[#2A2D3A] px-6 py-2.5 text-sm font-medium text-[#9DA1AB] transition-colors hover:border-[#C8920F] hover:text-[#C8920F]"
					style="font-family: 'DM Sans', sans-serif;"
				>
					Continue
				</button>
			</form>
		</div>
	</div>
</div>
