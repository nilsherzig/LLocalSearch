<script lang="ts">
	import { type LogElement, StepType } from '$lib/types/types';
	import { onMount } from 'svelte';
	import { marked } from 'marked';
	export let logElement: LogElement;
	export let isCurrentElement: boolean;
	export let showLogs: boolean;

	let collapsed = false;
	if (!isCurrentElement) {
		collapsed = true;
	}

	let scrollContainer: HTMLElement;
	function scollToElement() {
		if (scrollContainer === undefined) {
			return;
		}
		scrollContainer.scrollIntoView({ behavior: 'smooth', block: 'start' });
	}

	onMount(() => {
		scollToElement();
	});

	$: {
		console.log(logElement.message);
		scollToElement();
	}
</script>

<div
	class="
    w-full
    "
>
	<div
		bind:this={scrollContainer}
		id="level-{logElement.stepLevel}"
		class="break-words overflow-wrap"
	>
		<div class="max-w-prose">
			{#if showLogs == false}
				{#if logElement.stepType == StepType.HandleChainError || logElement.stepType == StepType.HandleToolError || logElement.stepType == StepType.HandleLlmError}
					<div
						id="level-{logElement.stepLevel}"
						class="rounded-lg shadow my-2 p-2 bg-stone-50 text-stone-600 border-red-500 border-2"
					>
						<span>{logElement.message}</span>
					</div>
				{:else if logElement.stepType == StepType.HandleFinalAnswer}
					<div
						class="rounded-lg shadow my-2 p-2 bg-stone-50 text-stone-950 border-stone-300 border-2"
					>
						<article class="p-2 prose prose-stone">
							{@html marked.parse(logElement.message)}
						</article>
					</div>
				{:else if logElement.stream}
					<div
						class="rounded-lg shadow my-2 p-2 bg-stone-50 text-stone-600 border-stone-300 border-2"
					>
						<span>{logElement.message}</span>
					</div>
				{/if}
				<!-- show all messages -->
			{:else if logElement.stepType == StepType.HandleChainError || logElement.stepType == StepType.HandleToolError || logElement.stepType == StepType.HandleLlmError}
				<div
					id="level-{logElement.stepLevel}"
					class="rounded-lg shadow my-2 p-2 bg-stone-50 text-stone-600 border-red-500 border-2"
				>
					<span>{logElement.message}</span>
				</div>
			{:else if logElement.stepType == StepType.HandleFinalAnswer}
				<div
					class="rounded-lg shadow my-2 p-2 bg-stone-50 text-stone-950 border-stone-300 border-2"
				>
					<article class="p-2 prose prose-stone">
						{@html marked.parse(logElement.message)}
					</article>
				</div>
			{:else if logElement.stepType == StepType.HandleToolStart}
				<div
					class="rounded-lg shadow my-2 p-2 bg-stone-50 text-stone-600 border-stone-300 border-2 flex justify-between"
				>
					<span>{logElement.message}</span>
				</div>
			{:else if logElement.stepType == StepType.HandleAgentAction}
				<div
					class="rounded-lg shadow my-2 p-2 bg-stone-50 text-stone-600 border-stone-300 border-2"
				>
					<span>{logElement.message}</span>
				</div>
			{:else if logElement.stepType == StepType.HandleChainStart}
				<div
					class="rounded-lg shadow my-2 p-2 bg-stone-50 text-stone-700 border-stone-300 border-2 flex justify-between"
				>
					<span>{logElement.message.slice(0, 200)}...</span>
				</div>
			{:else if logElement.stepType == StepType.HandleChainEnd}
				<div
					class="rounded-lg shadow my-2 p-2 bg-stone-50 text-stone-600 border-stone-300 border-2"
				>
					<span>{logElement.message}</span>
				</div>
			{:else if logElement.stepType == StepType.HandleSourceAdded}
				<div
					class="rounded-lg shadow my-2 p-2 bg-stone-50 text-stone-600 border-stone-300 border-2"
				>
					<a href={logElement.source.link} target="_blank">
						<img
							src="https://www.google.com/s2/favicons?domain={logElement.source.link}&sz={265}"
							class="rounded h-6 w-6 inline-block mr-2 filter grayscale"
							alt="favicon"
						/>
						<span>{logElement.source.link}</span>
						<span>{logElement.source.summary}</span>
					</a>
				</div>
			{:else}
				<div
					class="rounded-lg shadow my-2 p-2 bg-stone-50 text-stone-600 border-stone-300 border-2"
				>
					<span>{logElement.message}</span>
				</div>
			{/if}
		</div>
	</div>
</div>

<!-- <style lang="postcss"> -->
<!-- 	:global(#level-1) { -->
<!-- 		padding-left: 2rem; -->
<!-- 	} -->
<!-- 	:global(#level-2) { -->
<!-- 		padding-left: 4rem; -->
<!-- 	} -->
<!-- 	:global(#level-3) { -->
<!-- 		padding-left: 8rem; -->
<!-- 	} -->
<!-- </style> -->
