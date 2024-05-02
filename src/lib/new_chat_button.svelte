<script lang="ts">
	import { draw } from 'svelte/transition';
	import TopbarButton from './topbar_button.svelte';
	export let action: () => void;
</script>

<TopbarButton {action}>
	<svg
		xmlns="http://www.w3.org/2000/svg"
		fill="none"
		viewBox="0 0 24 24"
		stroke-width="1.2"
		stroke="currentColor"
		class="w-6 h-6"
	>
		<path
			in:draw={{ duration: 300 }}
			stroke-linecap="round"
			stroke-linejoin="round"
			d="M12 4.5v15m7.5-7.5h-15"
		/>
	</svg>
</TopbarButton>
