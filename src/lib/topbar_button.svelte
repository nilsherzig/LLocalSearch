<script lang="ts">
	import { fade } from 'svelte/transition';
	export let action: () => void;
	export let active = false;
	export let title: string;
</script>

<div in:fade={{ duration: 300 }}>
	<span {title}>
		{#if active}
			<button
				on:click={action}
				class="
            bg-neutral-200 dark:bg-neutral-800
            text-neutral-500 dark:text-neutral-400
            hover:text-neutral-700 dark:hover:text-neutral-300
            shadow-inner rounded-lg hover:cursor-pointer transition-all p-0.5"
			>
				<slot />
			</button>
		{:else}
			<button
				on:click={action}
				class="
            bg-neutral-100 dark:bg-neutral-900
            text-neutral-500 dark:text-neutral-400
            hover:text-neutral-700 dark:hover:text-neutral-300
            hover:bg-neutral-200 dark:hover:bg-neutral-800
            hover:shadow-inner hover:cursor-pointer
            rounded-lg transition-all p-0.5"
			>
				<slot />
			</button>
		{/if}
	</span>
</div>
