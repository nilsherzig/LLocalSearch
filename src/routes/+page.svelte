<script lang="ts">
	import { fade } from 'svelte/transition';
	import type { LogElement, ClientSettings as ClientValues } from '$lib/types/types';
	import { StepType } from '$lib/types/types';
	import { onDestroy, onMount } from 'svelte';
	import LogItem from '$lib/log_item.svelte';
	import BottomBar from '$lib/bottom_bar.svelte';
	import ToggleLogsButton from '$lib/toggle_logs_button.svelte';
	import ToggleDarkmodeButton from '$lib/toggle_darkmode_button.svelte';
	import SettingsWindow from '$lib/settings_window.svelte';
	import ToggleModelSwitch from '$lib/toggle_model_switch.svelte';

	let eventSource: EventSource | null = null;
	let sendMode = true;

	let models: string[];

	let skipSetLocalStorage = true;

	let scrollContainer: HTMLElement;

	let clientValues: ClientValues = {
		// default values, will be overwritten by local storage if available
		maxIterations: 30,
		contextSize: 8 * 1024,
		temperature: 0.0,
		modelName: 'adrienbrault/nous-hermes2pro:Q8_0',
		prompt: '',
		toolNames: [],
		webSearchCategories: [],
		session: 'default',
		amountOfResults: 4,
		minResultScore: 0.5,
		amountOfWebsites: 10,
		chunkSize: 300,
		chunkOverlap: 100
	};

	let showExamplePrompts = true;
	let examplePrompts = [
		'how much does a llama weigh?',
		'does openai work with the military?',
		'how much does obsidian sync cost?',
		'Pixel 7 camera specs',
		'whats up with the drama around apple and progressive web apps?',
		'how much do OpenAI and Microsoft plan to spend on their new datacenter?',
		'when is llama3 going to be releaesed?'
	];

	let lastElemWasStream = false;

	let logs: LogElement[] = [];

	let showLogs = false;
	let isDarkMode: boolean;
	let showSettings = false;
	let sessionString: string = 'default';

	function setLocalStorage(key: string, value: string) {
		if (typeof window === 'undefined') return;

		if (skipSetLocalStorage) {
			skipSetLocalStorage = false;
			return;
		}

		if (value === undefined) {
			return;
		}
		console.log('setting local storage', key, value);
		localStorage.setItem(key, value);
	}

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
	$: setLocalStorage('clientSettings', JSON.stringify(clientValues));

	onMount(() => {
		fetch('/api/models')
			.then((response) => response.json())
			.then((data) => {
				models = data;
			});

		if (localStorage.clientSettings) {
			clientValues = JSON.parse(localStorage.clientSettings);
			console.log('loaded client settings', clientValues);
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
		sessionString = 'default';
		showExamplePrompts = true;
	}

	function stopChat() {
		if (eventSource !== null) {
			eventSource.close();
		}
		eventSource = null;
	}

	// Establish a connection to the server-sent events endpoint
	function sendPrompt() {
		showExamplePrompts = false;

		let clientSettingsJsonString = JSON.stringify(clientValues);

		let url = '/api/stream?settings=' + encodeURIComponent(clientSettingsJsonString);

		let newLogElement: LogElement = {
			message: `${clientValues.prompt}`,
			stepType: StepType.HandleUserPrompt
		};

		logs.push(newLogElement);
		sendMode = false;
		clientValues.prompt = '';
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
			if (log.stepType == StepType.HandleOllamaStart) {
				let currentLogIndex = logs.length;
				setTimeout(() => {
					for (let i = currentLogIndex; i < logs.length; i++) {
						if (logs[i].stream) {
							return;
						}
					}
					let newLogElement: LogElement = {
						message: `Ollama is currently loading the model. This might take a few seconds.`,
						stepType: StepType.HandleOllamaModelLoadMessage
					};
					logs.push(newLogElement);
					logs = logs;
				}, 3000);
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
		};
		onMountHasRun = true;
	}
	onDestroy(() => {
		eventSource?.close();
	});

	function scollToElement(elem: LogElement) {
		if (scrollContainer === undefined) {
			return;
		}
		scrollContainer.scrollIntoView({ behavior: 'smooth', block: 'end' });
	}

	onMount(() => {
		scollToElement(logs[logs.length - 1]);
	});

	$: scollToElement(logs[logs.length - 1]);
</script>

<svelte:head>
	<link rel="preconnect" href="https://fonts.googleapis.com" />
	<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin="true" />
	<link
		href="https://fonts.googleapis.com/css2?family=Vollkorn:ital,wght@0,400..900;1,400..900&display=swap"
		rel="stylesheet"
	/>
</svelte:head>

<SettingsWindow bind:models bind:clientSettings={clientValues} bind:showSettings></SettingsWindow>
<div class="w-screen flex flex-col transition-all">
	<div class="px-2 flex items-center flex-col h-full overflow-scroll">
		<div class="py-24 align-middle" bind:this={scrollContainer}>
			{#if showExamplePrompts}
				<div in:fade out:fade class="flex flex-col gap-2 align-middle">
					{#each examplePrompts as examplePrompt}
						<div class="max-w-prose self-center">
							<button
								class="bg-stone-50 text-stone-700 py-2 px-6 rounded-lg shadow border-stone-300 border-2 hover:border-stone-400 transition-all dark:bg-stone-900 dark:border-stone-700 dark:text-stone-400 dark:hover:border-stone-500"
								tabindex="-1"
								on:click={() => {
									clientValues.prompt = examplePrompt;
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
<div class="fixed top-0 w-full rounded transition-all">
	<div
		class="flex p-4 justify-between dark:bg-stone-950 bg-stone-200 lg:bg-transparent lg:dark:bg-transparent lg:bg-gradient-to-b lg:from-stone-200 lg:to-transparent lg:dark:from-stone-950 transition-all"
	>
		<ToggleModelSwitch
			bind:showModelSwitchWindow={showSettings}
			bind:currentModel={clientValues.modelName}
		></ToggleModelSwitch>
		<!-- <div> -->
		<!-- 	<span class="text-stone-600 dark:text-stone-500 p-2"> -->
		<!-- 		{clientValues.modelName} -->
		<!-- 	</span> -->
		<!-- </div> -->
		<div class="flex flex-row">
			{#if logs.length > 0}
				<ToggleLogsButton bind:showLogs></ToggleLogsButton>
			{/if}
			<ToggleDarkmodeButton bind:isDarkMode></ToggleDarkmodeButton>
		</div>
	</div>
	<div class="lg:hidden bg-gradient-to-t from-transparent to-stone-200 dark:to-stone-950 h-4"></div>
</div>

<div
	class="fixed bottom-0 w-full transition-all bg-gradient-to-t from-stone-200 to-transparent dark:from-stone-950"
>
	<BottomBar
		bind:sendMode
		bind:eventSource
		bind:prompt={clientValues.prompt}
		{sendPrompt}
		{resetChat}
		{stopChat}
	></BottomBar>
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
