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
	<div class="break-words mx-4 overflow-wrap">
		<div class="max-w-prose flex flex-col">
			<!-- error -->
			{#if logElement.stepType == StepType.HandleChainError || logElement.stepType == StepType.HandleToolError || logElement.stepType == StepType.HandleLlmError || logElement.stepType == StepType.HandleParseError}
				<div
					class="rounded-lg shadow my-2 p-2 bg-neutral-50 text-neutral-600 border-red-600 border-2 dark:bg-neutral-800 dark:text-neutral-200"
				>
					{#if logElement.message.includes('Parsing Error.')}
						<span
							>Looks like the LLM didn't respond in the right format. It will try again. Consider
							using a LLM trained on structured output or function calling.</span
						>
					{:else}
						<article class="p-2 prose prose-neutral dark:prose-invert">
							{@html marked.parse(logElement.message)}
						</article>
					{/if}
				</div>
				<!-- ollama model load message -->
			{:else if logElement.stepType == StepType.HandleOllamaModelLoadMessage}
				<div class="flex text-neutral-500 dark:text-neutral-400 p-4 gap-4">
					{logElement.message}
				</div>
				<!-- user prompt  -->
			{:else if logElement.stepType == StepType.HandleUserMessage}
				<div class="border border-t-neutral-300 dark:border-neutral-800 mt-10 rounded"></div>
				<div
					class="my-2 p-2 text-neutral-700 dark:text-neutral-200 dark:border-neutral-400 font-semibold text-lg"
				>
					{logElement.message}
				</div>
				<!-- stream message -->
			{:else if logElement.stream || logElement.stepType == StepType.HandleFinalAnswer}
				{#if logElement.message.includes('Action: webscrape')}
					<div class="flex text-neutral-500 dark:text-neutral-400 p-4 gap-4">
						<svg
							xmlns="http://www.w3.org/2000/svg"
							fill="none"
							viewBox="0 0 24 24"
							stroke-width="1.2"
							stroke="currentColor"
							class="w-6 h-6"
						>
							<path
								in:draw={{ duration: 500 }}
								stroke-linecap="round"
								stroke-linejoin="round"
								d="M2.036 12.322a1.012 1.012 0 0 1 0-.639C3.423 7.51 7.36 4.5 12 4.5c4.638 0 8.573 3.007 9.963 7.178.07.207.07.431 0 .639C20.577 16.49 16.64 19.5 12 19.5c-4.638 0-8.573-3.007-9.963-7.178Z"
							/>
							<path
								stroke-linecap="round"
								stroke-linejoin="round"
								d="M15 12a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z"
							/>
						</svg>
						<span>{logElement.message.split('Action Input:')[1] || ''}</span>
					</div>
				{:else if logElement.message.includes('Action: database_search')}
					<div class="flex text-neutral-500 dark:text-neutral-400 p-4 gap-4">
						<svg
							xmlns="http://www.w3.org/2000/svg"
							fill="none"
							viewBox="0 0 24 24"
							stroke-width="1.2"
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
				{:else if logElement.message.includes('Action: websearch')}
					<div class="flex text-neutral-500 dark:text-neutral-400 p-4 gap-4">
						<svg
							xmlns="http://www.w3.org/2000/svg"
							fill="none"
							viewBox="0 0 24 24"
							stroke-width="1.2"
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
				{:else}
					<div
						class="rounded my-2 p-2 bg-neutral-100 text-neutral-600 border-neutral-300 border dark:bg-neutral-800 dark:text-neutral-300 dark:border-neutral-700"
					>
						<article class="p-2 prose prose-neutral dark:prose-invert">
							{@html marked.parse(
								logElement.message
									.replace('Thought: Do I need to use a tool? No\n', '')
									.replace('Thought: Do I need to use a tool? No', '')
									.replace('Do I need to use a tool? No', '')
									.replace('AI: ', '')
									.replace('AI:', '')
							)}
						</article>
						<!-- <span>{logElement.timeStamp}</span> -->
					</div>
				{/if}
			{/if}
			<!-- show all messages -->
			{#if showLogs}
				{#if logElement.stepType == StepType.HandleToolStart}
					<div
						class="rounded-lg shadow my-2 p-2 bg-neutral-100 text-neutral-500 border-neutral-300 border-2 dark:bg-neutral-900 dark:text-neutral-400 dark:border-neutral-700"
					>
						<span class="font-bold">Tool start</span>
						<span>{logElement.message}</span>
					</div>
				{:else if logElement.stepType == StepType.HandleAgentAction}
					<div
						class="rounded-lg shadow my-2 p-2 bg-neutral-100 text-neutral-500 border-neutral-300 border-2 dark:bg-neutral-900 dark:text-neutral-400 dark:border-neutral-700"
					>
						<span class="font-bold">Agent Action</span>
						<span>{logElement.message}</span>
					</div>
				{:else if logElement.stepType == StepType.HandleChainStart}
					<div
						class="rounded-lg shadow my-2 p-2 bg-neutral-100 text-neutral-500 border-neutral-300 border-2 dark:bg-neutral-900 dark:text-neutral-400 dark:border-neutral-700"
					>
						<span class="font-bold">Chain start</span>
						<span>{logElement.message}</span>
					</div>
				{:else if logElement.stepType == StepType.HandleChainEnd}
					<div
						class="rounded-lg shadow my-2 p-2 bg-neutral-100 text-neutral-500 border-neutral-300 border-2 dark:bg-neutral-900 dark:text-neutral-400 dark:border-neutral-700"
					>
						<span class="font-bold">Chain end</span>
						<span>{logElement.message}</span>
					</div>
				{/if}
			{/if}
		</div>
	</div>
</div>
