<script lang="ts">
	import { fade } from 'svelte/transition';
	export let action: () => void;
</script>

<div in:fade={{ duration: 300 }}>
	<button
		on:click={action}
		class="hover:bg-stone-100 hover:shadow text-stone-500 hover:text-stone-700 rounded-lg hover:cursor-pointer transition-all dark:text-stone-500 dark:hover:text-stone-300 dark:hover:bg-stone-900 p-0.5"
	>
		<slot />
	</button>
</div>
