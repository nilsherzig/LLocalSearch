<script lang="ts">
	import { fade } from 'svelte/transition';
	export let action: () => void;
	export let active = false;
</script>

<div in:fade={{ duration: 300 }}>
	{#if active}
		<button
			on:click={action}
			class="
            bg-stone-200 dark:bg-stone-800
            text-stone-500 dark:text-stone-400
            hover:text-stone-700 dark:hover:text-stone-300
            shadow-inner rounded-lg hover:cursor-pointer transition-all p-0.5"
		>
			<slot />
		</button>
	{:else}
		<button
			on:click={action}
			class="
            bg-stone-100 dark:bg-stone-900
            text-stone-500 dark:text-stone-400
            hover:text-stone-700 dark:hover:text-stone-300
            hover:bg-stone-200 dark:hover:bg-stone-800
            hover:shadow-inner hover:cursor-pointer
            rounded-lg transition-all p-0.5"
		>
			<slot />
		</button>
	{/if}
</div>
