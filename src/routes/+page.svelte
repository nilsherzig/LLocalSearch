<script lang="ts">
	import { fade, fly } from 'svelte/transition';
	import type { LogElement } from '$lib/types/types';
	import { StepType } from '$lib/types/types';
	import { onDestroy } from 'svelte';
	import LogItem from '$lib/log_item.svelte';
	import BottomBar from '$lib/bottom_bar.svelte';

	let eventSource: EventSource | null = null;
	let prompt = '';

	let showExamplePrompts = eventSource === null;
	$: showExamplePrompts = eventSource === null;
	let examplePrompts = [
		'how much does a llama weight?',
		'who was behind the terrorist attack in russia last week?',
		'how can i merge two kubectl config files?',
		'does openai work with the military?',
		'how much does obsidian sync cost?',
		'Pixel 7 camera specs',
		'whats up with the drama around apple and progressive web apps?',
		// Check censorship in the model.
		'How can i pirate nintendo switch games?',
		'Is china really killing innocent moslems?',
		'Is the police in the usa racist?'
	];

	let buttonText = 'Search';
	let lastElemWasStream = false;

	let logs: LogElement[] = [];
	let stepLevel = 0;
	let currentParent: LogElement;
	let lastParent: LogElement;

	let removeLevelNext = false;
	let addLevelNext = false;

	// Establish a connection to the server-sent events endpoint
	function toggleChat() {
		// stop current one if one exists
		if (eventSource !== null) {
			eventSource.close();
			eventSource = null;
			buttonText = 'Search';
			stepLevel = 0;
			removeLevelNext = false;
			addLevelNext = false;
			return;
		}

		// start a new one if null
		if (eventSource === null) {
			// clear logs
			logs = [];
			buttonText = 'Stop';
			// let url = 'http://localhost:8080/stream?prompt=' + prompt;
			let url = '/api?prompt=' + prompt;
			prompt = '';
			eventSource = new EventSource(url);
			console.log(eventSource);

			eventSource.onmessage = (event: MessageEvent) => {
				let log: LogElement = JSON.parse(event.data);
				if (!log.stream) {
					console.log(log);
				}
				if (removeLevelNext) {
					stepLevel -= 1;
					removeLevelNext = false;
				}

				if (addLevelNext) {
					stepLevel += 1;
					addLevelNext = false;
				}

				if (log.close) {
					eventSource?.close();
					eventSource = null;
					return;
				}

				if (log.stepType) {
					if (
						log.stepType == StepType.HandleChainStart ||
						log.stepType == StepType.HandleToolStart ||
						log.stepType == StepType.HandleLlmStart
					) {
						addLevelNext = true;
						if (lastParent) {
							lastParent = currentParent;
						}
						currentParent = log;
					}
					if (
						log.stepType == StepType.HandleChainEnd ||
						log.stepType == StepType.HandleToolEnd ||
						log.stepType == StepType.HandleChainError
					) {
						currentParent = lastParent;
						removeLevelNext = true;
					}
					console.log(stepLevel);
					// console.log(currentParent);
				}

				log.stepLevel = stepLevel;

				if (log.message) {
					log.message = log.message.replaceAll('<|im_end|>', '');
					if (log.stream) {
						if (lastElemWasStream) {
							logs[logs.length - 1].message += log.message;
						} else {
							logs.push(log);
						}
						lastElemWasStream = true;
					} else {
						lastElemWasStream = false;
						logs.push(log);
					}
				}

				logs = logs;
			};

			eventSource.onerror = (error: Event) => {
				console.error('EventSource failed:', error);
				eventSource?.close();
				eventSource = null;
				// Optionally, implement reconnection logic here
			};
			// console.log(steps);
		}
	}
	onDestroy(() => {
		eventSource?.close();
	});
</script>

<svelte:head>
	<link rel="preconnect" href="https://fonts.googleapis.com" />
	<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin />
	<link
		href="https://fonts.googleapis.com/css2?family=Vollkorn:ital,wght@0,400..900;1,400..900&display=swap"
		rel="stylesheet"
	/>
</svelte:head>

<div class="w-screen h-screen flex flex-col">
	<div class="px-2 flex items-center flex-col h-full overflow-scroll">
		<div class="py-24 align-middle">
			{#if showExamplePrompts}
				<div in:fade out:fade class="flex flex-col gap-2 align-middle">
					{#each examplePrompts as examplePrompt}
						<div class="max-w-prose self-center">
							<button
								class="bg-stone-50 text-stone-700 py-2 px-6 rounded-lg shadow shadow-stone-300 border-stone-300 border-2 hover:border-stone-400 transition-all"
								tabindex="-1"
								on:click={() => {
									prompt = examplePrompt;
									toggleChat();
								}}
							>
								{examplePrompt}
							</button>
						</div>
					{/each}
				</div>
			{/if}
			{#each logs as log, index}
				<div in:fade>
					{#if index == logs.length - 1}
						<div>
							<LogItem logElement={log} isCurrentElement={true}></LogItem>
						</div>
					{:else}
						<LogItem logElement={log} isCurrentElement={false}></LogItem>
					{/if}
				</div>
			{/each}
		</div>
	</div>
</div>
<div class="absolute h-10 top-0 w-full bg-gradient-to-b to-transparent from-stone-200 rounded">
	<div class="flex pt-8 px-8 justify-between bg-gradient-to-b from-stone-100 to-transparent">
		<div></div>
		<div
			class="hover:bg-stone-300 text-stone-500 hover:text-stone-700 rounded-2xl hover:cursor-pointer"
		>
			<div class="hover:rotate-90 p-2 transition-all">
				<svg
					xmlns="http://www.w3.org/2000/svg"
					fill="none"
					viewBox="0 0 24 24"
					stroke-width="1.5"
					stroke="currentColor"
					class="w-6 h-6"
				>
					<path
						stroke-linecap="round"
						stroke-linejoin="round"
						d="M9.594 3.94c.09-.542.56-.94 1.11-.94h2.593c.55 0 1.02.398 1.11.94l.213 1.281c.063.374.313.686.645.87.074.04.147.083.22.127.325.196.72.257 1.075.124l1.217-.456a1.125 1.125 0 0 1 1.37.49l1.296 2.247a1.125 1.125 0 0 1-.26 1.431l-1.003.827c-.293.241-.438.613-.43.992a7.723 7.723 0 0 1 0 .255c-.008.378.137.75.43.991l1.004.827c.424.35.534.955.26 1.43l-1.298 2.247a1.125 1.125 0 0 1-1.369.491l-1.217-.456c-.355-.133-.75-.072-1.076.124a6.47 6.47 0 0 1-.22.128c-.331.183-.581.495-.644.869l-.213 1.281c-.09.543-.56.94-1.11.94h-2.594c-.55 0-1.019-.398-1.11-.94l-.213-1.281c-.062-.374-.312-.686-.644-.87a6.52 6.52 0 0 1-.22-.127c-.325-.196-.72-.257-1.076-.124l-1.217.456a1.125 1.125 0 0 1-1.369-.49l-1.297-2.247a1.125 1.125 0 0 1 .26-1.431l1.004-.827c.292-.24.437-.613.43-.991a6.932 6.932 0 0 1 0-.255c.007-.38-.138-.751-.43-.992l-1.004-.827a1.125 1.125 0 0 1-.26-1.43l1.297-2.247a1.125 1.125 0 0 1 1.37-.491l1.216.456c.356.133.751.072 1.076-.124.072-.044.146-.086.22-.128.332-.183.582-.495.644-.869l.214-1.28Z"
					/>
					<path
						stroke-linecap="round"
						stroke-linejoin="round"
						d="M15 12a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z"
					/>
				</svg>
			</div>
		</div>
	</div>
</div>
<div class="absolute bottom-0 w-full bg-gradient-to-t to-transparent from-stone-200 rounded">
	<BottomBar bind:prompt {toggleChat}></BottomBar>
</div>

<style lang="postcss">
	:global(html) {
		background-color: theme(colors.stone.200);
		font-family: 'Vollkorn', serif;
	}
</style>
