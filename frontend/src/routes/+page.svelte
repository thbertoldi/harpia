<script lang="ts">
	import { Braces } from 'lucide-svelte';
</script>

<main class="flex min-h-screen flex-col items-center justify-center gap-6 p-8">
	<div class="flex items-center gap-3">
		<Braces class="size-10 text-primary" />
		<h1 class="text-4xl font-bold tracking-tight">Harpia</h1>
	</div>

	<p class="max-w-md text-center text-lg text-muted-foreground">
		Multi-tenant, multi-agent AI task management platform.
	</p>

	<div class="flex gap-4">
		<span class="inline-flex items-center rounded-full border px-3 py-1 text-xs font-medium">
			API: Online
		</span>
		<span class="inline-flex items-center rounded-full border px-3 py-1 text-xs font-medium">
			Agents: Idle
		</span>
	</div>
</main>
