<script lang="ts">
	import { fade } from 'svelte/transition';
	export let action: () => void;
	export let active = false;
</script>

<div in:fade={{ duration: 300 }}>
	{#if active}
		<button
			on:click={action}
			class="hover:bg-stone-300 bg-stone-300 shadow-inner hover:shadow-inner text-stone-500 hover:text-stone-700 rounded-lg hover:cursor-pointer transition-all dark:text-stone-500 dark:hover:text-stone-300 dark:hover:bg-stone-900 p-0.5"
		>
			<slot />
		</button>
	{:else}
		<button
			on:click={action}
			class="hover:bg-stone-300 hover:shadow-inner text-stone-500 hover:text-stone-700 rounded-lg hover:cursor-pointer transition-all dark:text-stone-500 dark:hover:text-stone-300 dark:hover:bg-stone-900 p-0.5"
		>
			<slot />
		</button>
	{/if}
</div>
