<script lang="ts">
	import { StepType, type LogElement } from '$lib/types/types';
	export let chatHistoryItems: LogElement[];
</script>

<div class="flex flex-col gap-2 h-full">
	{#each chatHistoryItems as chatHistoryItem}
		{#if chatHistoryItem.stepType == StepType.HandleStreaming}
			<div class="rounded-lg bg-stone-50 shadow p-2 max-w-prose border-stone-300 border-2">
				<span class="text-green-900">{chatHistoryItem.stepType}:</span>
				<span>{chatHistoryItem.message}</span>
			</div>
		{/if}
	{/each}
</div>
