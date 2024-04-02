<script lang="ts">
	import { type LogElement, StepType } from '$lib/types/types';
	import { onMount } from 'svelte';
	import { marked } from 'marked';
	export let logElement: LogElement;
	export let showLogs: boolean;

	let scrollContainer: HTMLElement;
	function scollToElement(elem: LogElement) {
		if (scrollContainer === undefined) {
			return;
		}
		scrollContainer.scrollIntoView({ behavior: 'smooth', block: 'start' });
		if (elem.stepType == StepType.HandleFinalAnswer) {
		}
	}

	onMount(() => {
		scollToElement(logElement);
	});

	// $: {
	// 	console.log(logElement.stepType);
	// 	scollToElement();
	// }
	$: scollToElement(logElement);
	const renderer = {
		link(href: string, title: string, text: string) {
			const link = marked.Renderer.prototype.link.call(this, href, title, text);
			return link.replace('<a', "<a target='_blank' rel='noopener noreferrer' ");
		}
	};

	marked.use({
		renderer
	});
</script>

<div
	class="
    w-full
    "
>
	<div bind:this={scrollContainer} class="break-words overflow-wrap">
		<div class="max-w-prose flex flex-col">
			<!-- error -->
			{#if logElement.stepType == StepType.HandleChainError || logElement.stepType == StepType.HandleToolError || logElement.stepType == StepType.HandleLlmError || logElement.stepType == StepType.HandleParseError}
				<div
					class="rounded-lg shadow my-2 p-2 bg-stone-50 text-stone-600 border-red-600 border-2 dark:bg-stone-800 dark:text-stone-200 mr-10"
				>
					<span>{logElement.message}</span>
				</div>
				<!-- user prompt -->
			{:else if logElement.stepType == StepType.HandleUserPrompt}
				<div
					class="self-end rounded-lg shadow my-2 p-2 bg-stone-50 text-stone-700 border-stone-400 border-2 dark:bg-stone-800 dark:text-stone-200 dark:border-stone-400 ml-10"
				>
					{logElement.message}
				</div>
				<!-- final answer -->
			{:else if logElement.stepType == StepType.HandleFinalAnswer}
				<div
					class="rounded-lg shadow my-2 p-2 bg-stone-50 text-stone-950 border-stone-300 border-2 dark:bg-stone-800 dark:text-stone-200 dark:border-stone-700 mr-10"
				>
					<article class="p-2 prose prose-stone dark:prose-invert">
						{@html marked.parse(logElement.message)}
					</article>
				</div>
				<!-- stream message -->
			{:else if logElement.stream}
				<div
					class="rounded-lg shadow my-2 p-2 bg-stone-100 text-stone-600 border-stone-300 border-2 dark:bg-stone-800 dark:text-stone-300 dark:border-stone-700 mr-10"
				>
					<article class="p-2 prose prose-stone dark:prose-invert">
						{@html marked.parse(logElement.message)}
						<!-- <span>{logElement.message}</span> -->
					</article>
				</div>
			{/if}
			<!-- show all messages -->
			{#if showLogs}
				{#if logElement.stepType == StepType.HandleToolStart}
					<div
						class="rounded-lg shadow my-2 p-2 bg-stone-100 text-stone-500 border-stone-300 border-2 dark:bg-stone-900 dark:text-stone-400 dark:border-stone-700"
					>
						<span class="font-bold">Tool start</span>
						<span>{logElement.message}</span>
					</div>
				{:else if logElement.stepType == StepType.HandleAgentAction}
					<div
						class="rounded-lg shadow my-2 p-2 bg-stone-100 text-stone-500 border-stone-300 border-2 dark:bg-stone-900 dark:text-stone-400 dark:border-stone-700"
					>
						<span class="font-bold">Agent Action</span>
						<span>{logElement.message}</span>
					</div>
				{:else if logElement.stepType == StepType.HandleChainStart}
					<div
						class="rounded-lg shadow my-2 p-2 bg-stone-100 text-stone-500 border-stone-300 border-2 dark:bg-stone-900 dark:text-stone-400 dark:border-stone-700"
					>
						<span class="font-bold">Chain start</span>
						<span>{logElement.message}</span>
					</div>
				{:else if logElement.stepType == StepType.HandleChainEnd}
					<div
						class="rounded-lg shadow my-2 p-2 bg-stone-100 text-stone-500 border-stone-300 border-2 dark:bg-stone-900 dark:text-stone-400 dark:border-stone-700"
					>
						<span class="font-bold">Chain end</span>
						<span>{logElement.message}</span>
					</div>
				{:else if logElement.stepType == StepType.HandleSourceAdded}
					<div
						class="rounded-lg shadow my-2 p-2 bg-stone-100 text-stone-500 border-stone-300 border-2 dark:bg-stone-900 dark:text-stone-400 dark:border-stone-700"
					>
						<span class="font-bold">Source added</span>
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
				{/if}
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
