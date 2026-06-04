<script lang="ts">
	let { children, ...rest }: {
		children: import('svelte').Snippet;
		[key: string]: unknown;
	} = $props();
</script>

<div
	class="rounded-lg border bg-card text-card-foreground shadow-sm p-6"
	{...rest}
>
	{@render children()}
</div>
