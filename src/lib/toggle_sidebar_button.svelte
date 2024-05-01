<script lang="ts">
	import { draw } from 'svelte/transition';
	import TopbarButton from './topbar_button.svelte';

	export let showSidebar: boolean;
</script>

{#if showSidebar}
	<TopbarButton action={() => (showSidebar = false)}>
		<svg
			xmlns="http://www.w3.org/2000/svg"
			fill="none"
			viewBox="0 0 24 24"
			stroke-width="1.5"
			stroke="currentColor"
			class="w-6 h-6"
		>
			<path
				in:draw={{ duration: 300 }}
				stroke-linecap="round"
				stroke-linejoin="round"
				d="M8.25 9V5.25A2.25 2.25 0 0 1 10.5 3h6a2.25 2.25 0 0 1 2.25 2.25v13.5A2.25 2.25 0 0 1 16.5 21h-6a2.25 2.25 0 0 1-2.25-2.25V15m-3 0-3-3m0 0 3-3m-3 3H15"
			/>
		</svg>
	</TopbarButton>
{:else}
	<TopbarButton action={() => (showSidebar = true)}>
		<svg
			xmlns="http://www.w3.org/2000/svg"
			fill="none"
			viewBox="0 0 24 24"
			stroke-width="1.5"
			stroke="currentColor"
			class="w-6 h-6"
		>
			<path
				in:draw={{ duration: 300 }}
				stroke-linecap="round"
				stroke-linejoin="round"
				d="M15.75 9V5.25A2.25 2.25 0 0 0 13.5 3h-6a2.25 2.25 0 0 0-2.25 2.25v13.5A2.25 2.25 0 0 0 7.5 21h6a2.25 2.25 0 0 0 2.25-2.25V15m3 0 3-3m0 0-3-3m3 3H9"
			/>
		</svg>
	</TopbarButton>
{/if}
