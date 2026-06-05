<script lang="ts">
	let { children, ...rest }: {
		children: import('svelte').Snippet;
		[key: string]: unknown;
	} = $props();
</script>

<div
	class="rounded-lg border border-plumage bg-obsidian-light text-cream shadow-sm p-6"
	{...rest}
>
	{@render children()}
</div>
