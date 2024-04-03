<script lang="ts">
	import TopButton from './top_button.svelte';

	export let showModelSwitchWindow: boolean;
	export let currentModel: string;
	let cleanName = (name: string) => {
		if (name.includes(':')) {
			name = name.split(':')[0];
		}
		if (name.includes('/')) {
			name = name.split('/')[1];
		}
		return name;
	};
</script>

{#if currentModel}
	<TopButton>
		<span title="change llm">
			<button on:click={() => (showModelSwitchWindow = !showModelSwitchWindow)} class="p-2"
				>{cleanName(currentModel)}</button
			>
		</span>
	</TopButton>
{/if}
