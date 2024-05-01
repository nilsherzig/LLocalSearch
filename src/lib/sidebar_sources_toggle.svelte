<script lang="ts">
	import { draw } from 'svelte/transition';
	import TopbarButton from './topbar_button.svelte';
	export let rightBarMode: string;
	$: console.log(rightBarMode);
	let active = () => {
		return rightBarMode == 'sources';
	};
</script>

<span title="hide logs">
	<TopbarButton action={() => (rightBarMode = 'sources')} active={rightBarMode == 'sources'}>
		<svg
			xmlns="http://www.w3.org/2000/svg"
			fill="none"
			viewBox="0 0 24 24"
			stroke-width="1.5"
			stroke="currentColor"
			class="w-6 h-6"
		>
			<path
				in:draw={{ duration: 500 }}
				stroke-linecap="round"
				stroke-linejoin="round"
				d="M13.19 8.688a4.5 4.5 0 0 1 1.242 7.244l-4.5 4.5a4.5 4.5 0 0 1-6.364-6.364l1.757-1.757m13.35-.622 1.757-1.757a4.5 4.5 0 0 0-6.364-6.364l-4.5 4.5a4.5 4.5 0 0 0 1.242 7.244"
			/>
		</svg>
	</TopbarButton>
</span>
