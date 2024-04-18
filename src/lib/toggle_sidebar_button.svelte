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
				d="M15.75 19.5 8.25 12l7.5-7.5"
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
				d="m8.25 4.5 7.5 7.5-7.5 7.5"
			/>
		</svg>
	</TopbarButton>
{/if}
