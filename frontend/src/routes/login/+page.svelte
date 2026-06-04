<script lang="ts">
	import { goto } from '$app/navigation';
	import { login, getSession } from '$lib/auth';

	let email = $state('');

	$effect(() => {
		if (getSession()) {
			goto('/');
		}
	});

	async function handleLogin() {
		await login(email ? { login_hint: email } : undefined);
	}
</script>

<div class="flex min-h-screen items-center justify-center bg-[#121318]">
	<div class="fixed left-0 top-0 h-full w-[5px] bg-gradient-to-b from-[#E0A512] via-[#C8920F] to-[#C8920F]/30"></div>

	<div class="w-full max-w-md space-y-8 p-8">
		<div class="text-center">
			<h1 class="font-heading text-5xl font-black text-[#C8920F] tracking-tight" style="font-family: 'Bodoni Moda', serif">
				Harpia
			</h1>
			<p class="mt-3 font-body text-[#9DA1AB]" style="font-family: 'DM Sans', sans-serif">
				AI operations for your business
			</p>
		</div>

		<div class="space-y-4">
			<button
				onclick={handleLogin}
				class="w-full cursor-pointer rounded-lg bg-[#C8920F] px-6 py-3 font-medium text-[#121318] transition-all hover:bg-[#E0A512]"
				style="font-family: 'DM Sans', sans-serif"
			>
				Sign in with Zitadel
			</button>

			<div class="relative">
				<div class="absolute inset-0 flex items-center">
					<div class="w-full border-t border-[#2A2D3A]"></div>
				</div>
				<div class="relative flex justify-center text-xs">
					<span class="bg-[#121318] px-2 font-mono text-[#6B7080] uppercase tracking-wider" style="font-family: 'JetBrains Mono', monospace">
						or
					</span>
				</div>
			</div>

			<form onsubmit={(e) => { e.preventDefault(); handleLogin(); }} class="space-y-3">
				<input
					type="email"
					bind:value={email}
					placeholder="admin@harpia.local"
					class="w-full rounded-lg border border-[#2A2D3A] bg-[#1A1B24] px-4 py-3 font-body text-[#F5F2EB] placeholder:text-[#6B7080] focus:border-[#C8920F] focus:outline-none transition-colors"
					style="font-family: 'DM Sans', sans-serif"
				/>
				<button
					type="submit"
					class="w-full cursor-pointer rounded-lg border border-[#2A2D3A] px-6 py-3 font-body text-sm text-[#9DA1AB] transition-colors hover:border-[#C8920F] hover:text-[#C8920F]"
					style="font-family: 'DM Sans', sans-serif"
				>
					Continue with email
				</button>
			</form>
		</div>
	</div>
</div>
