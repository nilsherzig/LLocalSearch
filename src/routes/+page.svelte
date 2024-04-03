<script lang="ts">
	import { fade } from 'svelte/transition';
	import type { LogElement } from '$lib/types/types';
	import { StepType } from '$lib/types/types';
	import { onDestroy, onMount } from 'svelte';
	import LogItem from '$lib/log_item.svelte';
	import BottomBar from '$lib/bottom_bar.svelte';
	import ToggleLogsButton from '$lib/toggle_logs_button.svelte';
	import ToggleDarkmodeButton from '$lib/toggle_darkmode_button.svelte';
	import ModelSwitchWindow from '$lib/model_switch_window.svelte';
	import ToggleModelSwitch from '$lib/toggle_model_switch.svelte';

	let eventSource: EventSource | null = null;
	let prompt = '';
	let sendMode = true;

	let showModelSwitchWindow = false;
	let models: string[];

	let currentModel: string;

	$: setLocalStorage('currentModel', currentModel);

	function setLocalStorage(key: string, value: string) {
		if (typeof window === 'undefined') return;
		if (value === undefined) {
			return;
		}
		console.log('setting local storage', key, value);
		localStorage.setItem(key, value);
	}

	let showExamplePrompts = true;
	let examplePrompts = [
		'how much does a llama weight?',
		'does openai work with the military?',
		'how much does obsidian sync cost?',
		'Pixel 7 camera specs',
		'whats up with the drama around apple and progressive web apps?',
		'how much do OpenAI and Microsoft plan to spend on their new datacenter?'
	];

	let lastElemWasStream = false;

	let logs: LogElement[] = [];

	let showLogs = false;
	let isDarkMode: boolean;
	let sessionString: string = '';

	function changeDarkMode(isDarkMode: boolean) {
		if (typeof window === 'undefined') return;
		if (isDarkMode === undefined) {
			isDarkMode =
				localStorage.theme === 'dark' ||
				(!('theme' in localStorage) && window.matchMedia('(prefers-color-scheme: dark)').matches);
		}
		if (isDarkMode) {
			document.documentElement.classList.add('dark');
			localStorage.theme = 'dark';
		} else {
			document.documentElement.classList.remove('dark');
			localStorage.theme = 'light';
		}
	}

	$: changeDarkMode(isDarkMode);

	onMount(() => {
		fetch('/api/modellist')
			.then((response) => response.json())
			.then((data) => {
				models = data;
			});

		if (localStorage.currentModel) {
			currentModel = localStorage.currentModel;
		} else {
			currentModel = 'knoopx/hermes-2-pro-mistral:7b-q8_0'; // default model
		}
		if (
			localStorage.theme === 'dark' ||
			(!('theme' in localStorage) && window.matchMedia('(prefers-color-scheme: dark)').matches)
		) {
			document.documentElement.classList.add('dark');
			isDarkMode = true;
		} else {
			document.documentElement.classList.remove('dark');
			isDarkMode = false;
		}
	});

	function resetChat() {
		if (eventSource !== null) {
			eventSource.close();
		}
		eventSource = null;
		logs = [];
		sessionString = '';
		showExamplePrompts = true;
	}

	// Establish a connection to the server-sent events endpoint
	function sendPrompt() {
		showExamplePrompts = false;
		let url =
			'/api/stream?prompt=' + prompt + '&session=' + sessionString + '&modelname=' + currentModel;
		let newLogElement: LogElement = {
			message: `${prompt}`,
			stepType: StepType.HandleUserPrompt
		};
		logs.push(newLogElement);
		sendMode = false;
		prompt = '';
		logs = logs;
		eventSource = new EventSource(url);
		eventSource.onmessage = (event: MessageEvent) => {
			let log: LogElement = JSON.parse(event.data);
			if (log.stepType == StepType.HandleNewSession) {
				console.log('new session', log.session);
				sessionString = log.session;
				return;
			}
			if (log.close) {
				eventSource?.close();
				eventSource = null;
				console.log('closing event source');
				sendMode = true;
				return;
			}

			if (log.message) {
				log.message = log.message.replaceAll('<|im_end|>', '').replaceAll('<|end_of_turn|>', '');
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
			sendMode = true;
			// Optionally, implement reconnection logic here
		};
		// console.log(steps);
	}
	onDestroy(() => {
		eventSource?.close();
	});
</script>

<svelte:head>
	<link rel="preconnect" href="https://fonts.googleapis.com" />
	<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin="true" />
	<link
		href="https://fonts.googleapis.com/css2?family=Vollkorn:ital,wght@0,400..900;1,400..900&display=swap"
		rel="stylesheet"
	/>
</svelte:head>

<ModelSwitchWindow bind:models bind:showModelSwitchWindow bind:currentModel></ModelSwitchWindow>
<div class="w-screen h-screen flex flex-col transition-all">
	<div class="px-2 flex items-center flex-col h-full overflow-scroll">
		<div class="py-24 align-middle">
			{#if showExamplePrompts}
				<div in:fade out:fade class="flex flex-col gap-2 align-middle">
					{#each examplePrompts as examplePrompt}
						<div class="max-w-prose self-center">
							<button
								class="bg-stone-50 text-stone-700 py-2 px-6 rounded-lg shadow border-stone-300 border-2 hover:border-stone-400 transition-all dark:bg-stone-900 dark:border-stone-700 dark:text-stone-400 dark:hover:border-stone-500"
								tabindex="-1"
								on:click={() => {
									prompt = examplePrompt;
									sendPrompt();
								}}
							>
								{examplePrompt}
							</button>
						</div>
					{/each}
				</div>
			{/if}
			{#each logs as log}
				<div in:fade>
					<LogItem logElement={log} bind:showLogs></LogItem>
				</div>
			{/each}
		</div>
	</div>
</div>
<div class="absolute top-0 w-full rounded transition-all">
	<div class="flex p-4 justify-between bg-stone-200 dark:bg-stone-950 transition-all">
		<div>
			<ToggleModelSwitch bind:currentModel bind:showModelSwitchWindow></ToggleModelSwitch>
		</div>
		<div class="flex flex-row">
			<!-- dont show log toggle when there are no logs -->
			{#if logs.length > 0}
				<ToggleLogsButton bind:showLogs></ToggleLogsButton>
			{/if}
			<ToggleDarkmodeButton bind:isDarkMode></ToggleDarkmodeButton>
		</div>
	</div>
	<div class="bg-gradient-to-t from-transparent to-stone-200 dark:to-stone-950 h-4"></div>
</div>

<div
	class="absolute bottom-0 w-full transition-all bg-gradient-to-t from-stone-200 to-transparent dark:from-stone-950"
>
	<BottomBar bind:sendMode bind:prompt {sendPrompt} {resetChat}></BottomBar>
</div>

<style lang="postcss">
	:global(html) {
		background-color: theme(colors.stone.200);
		font-family: 'Vollkorn', serif;
	}
	:global(html.dark) {
		background-color: theme(colors.stone.950);
		font-family: 'Vollkorn', serif;
	}
</style>
