<script lang="ts">
	import { fade } from 'svelte/transition';
	export let showModelSwitchWindow: boolean;
	export let models: string[];
	export let currentModel: string;
</script>

{#if showModelSwitchWindow}
	<div in:fade={{ duration: 300 }} out:fade={{ duration: 300 }} class="absolute z-50">
		<div
			class="m-4 max-w-fit bg-stone-200 p-2 rounded-lg shadow-lg border-2 border-stone-300 dark:bg-stone-950 dark:border-stone-800 dark:text-stone-300 max-h-72 overflow-scroll"
		>
			<p class="font-bold"></p>
			<div class="flex flex-col">
				{#each models as model}
					<button
						on:click={() => {
							currentModel = model;
							showModelSwitchWindow = false;
						}}
					>
						<p
							class="bg-stone-50 border-stone-300 hover:border-stone-400 rounded-lg px-2 py-1.5 my-1 hover:cursor-pointer border-2 dark:border-stone-700 dark:hover:border-stone-600 dark:bg-stone-800 transition-all shadow"
						>
							{#if model === currentModel}
								<span class="font-bold">{model}</span>
							{:else}
								{model}
							{/if}
						</p>
					</button>
				{/each}
			</div>
		</div>
	</div>
{/if}
