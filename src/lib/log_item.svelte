<script lang="ts">
	import { draw } from 'svelte/transition';
	import { type LogElement, StepType } from '$lib/types/types';
	import { marked } from 'marked';
	export let logElement: LogElement;
	export let showLogs: boolean;

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
	<div class="break-words overflow-wrap">
		<div class="max-w-prose flex flex-col">
			<!-- error -->
			{#if logElement.stepType == StepType.HandleChainError || logElement.stepType == StepType.HandleToolError || logElement.stepType == StepType.HandleLlmError || logElement.stepType == StepType.HandleParseError}
				<div
					class="rounded-lg shadow my-2 p-2 bg-stone-50 text-stone-600 border-red-600 border-2 dark:bg-stone-800 dark:text-stone-200"
				>
					<span>{logElement.message}</span>
				</div>
				<!-- ollama model load message -->
			{:else if logElement.stepType == StepType.HandleOllamaModelLoadMessage}
				<div class="flex text-stone-500 dark:text-stone-400 p-4 gap-4">
					{logElement.message}
				</div>
			{:else if logElement.stepType == StepType.HandleUserPrompt}
				<div class="w-full border-2 border-t-stone-300 dark:border-stone-800 mt-10 rounded"></div>
				<div
					class="my-2 p-2 text-stone-700 dark:text-stone-200 dark:border-stone-400 font-bold text-lg"
				>
					{logElement.message}
				</div>
				<!-- stream message -->
			{:else if logElement.stream}
				{#if logElement.message.includes('Action: WebSearch')}
					<div class="flex text-stone-500 dark:text-stone-400 p-4 gap-4">
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
								d="M12 21a9.004 9.004 0 0 0 8.716-6.747M12 21a9.004 9.004 0 0 1-8.716-6.747M12 21c2.485 0 4.5-4.03 4.5-9S14.485 3 12 3m0 18c-2.485 0-4.5-4.03-4.5-9S9.515 3 12 3m0 0a8.997 8.997 0 0 1 7.843 4.582M12 3a8.997 8.997 0 0 0-7.843 4.582m15.686 0A11.953 11.953 0 0 1 12 10.5c-2.998 0-5.74-1.1-7.843-2.918m15.686 0A8.959 8.959 0 0 1 21 12c0 .778-.099 1.533-.284 2.253m0 0A17.919 17.919 0 0 1 12 16.5c-3.162 0-6.133-.815-8.716-2.247m0 0A9.015 9.015 0 0 1 3 12c0-1.605.42-3.113 1.157-4.418"
							/>
						</svg>
						<span>{logElement.message.split('Action Input:')[1] || ''}</span>
					</div>
				{:else if logElement.message.includes('Action: SearchVectorDB')}
					<div class="flex text-stone-500 dark:text-stone-400 p-4 gap-4">
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
								d="M20.25 6.375c0 2.278-3.694 4.125-8.25 4.125S3.75 8.653 3.75 6.375m16.5 0c0-2.278-3.694-4.125-8.25-4.125S3.75 4.097 3.75 6.375m16.5 0v11.25c0 2.278-3.694 4.125-8.25 4.125s-8.25-1.847-8.25-4.125V6.375m16.5 0v3.75m-16.5-3.75v3.75m16.5 0v3.75C20.25 16.153 16.556 18 12 18s-8.25-1.847-8.25-4.125v-3.75m16.5 0c0 2.278-3.694 4.125-8.25 4.125s-8.25-1.847-8.25-4.125"
							/>
						</svg>
						<span>{logElement.message.split('Action Input:')[1] || ''}</span>
					</div>
				{:else}
					<div
						class="rounded-lg shadow my-2 p-2 bg-stone-100 text-stone-600 border-stone-300 border-2 dark:bg-stone-800 dark:text-stone-300 dark:border-stone-700"
					>
						<article class="p-2 prose prose-stone dark:prose-invert">
							{@html marked.parse(
								logElement.message
									.replaceAll('AI:', '')
									.replaceAll('Do I need to use a tool? No', '')
							)}
						</article>
					</div>
				{/if}
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
