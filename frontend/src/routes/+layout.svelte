<script lang="ts">
	import '../app.css';

	let { children } = $props();
</script>

<svelte:head>
	<script>
		(() => {
			const theme = localStorage.getItem('mode-watcher-theme') ?? 'system';
			const dark = theme === 'dark' || (theme === 'system' && window.matchMedia('(prefers-color-scheme: dark)').matches);
			document.documentElement.classList.toggle('dark', dark);
		})();
	</script>
</svelte:head>

{@render children()}
