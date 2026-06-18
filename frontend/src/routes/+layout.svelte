<script lang="ts">
import '../app.css';

let { children } = $props();
</script>

<svelte:head>
	<title>Sampo</title>
</svelte:head>

{@render children()}
