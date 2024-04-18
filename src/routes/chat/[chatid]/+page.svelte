<script lang="ts">
	import ChatList from '$lib/chatList.svelte';
	import { fade, slide } from 'svelte/transition';
	import type { PageData } from './$types';
	import { onDestroy, onMount } from 'svelte';
	import ToggleSidebarButton from '$lib/toggle_sidebar_button.svelte';
	import LoadingMessage from '$lib/loading_message.svelte';
	import { type LogElement, type Source, type ClientSettings, StepType } from '$lib/types/types';
	import LogItem from '$lib/log_item.svelte';
	import { changeDarkMode } from './handle_darkmode';
	import BottomBar from '$lib/bottom_bar.svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import NewChatButton from '$lib/new_chat_button.svelte';
	import ShowLogsButton from '$lib/show_logs_button.svelte';
	import ToggleDarkmodeButton from '$lib/toggle_darkmode_button.svelte';

	export let showLogs = false;
	export let data: PageData;
	let currentLogs: LogElement[] = [];
	let lastLogitemWasStream = false;
	let scrollContainer: HTMLElement;

	let eventSource: EventSource | null = null;
	let sendMode = true;
	let searchSources: Source[] = [];
	const defaultClientValues: ClientSettings = {
		// default values, will be overwritten by local storage if available
		maxIterations: 30,
		contextSize: 8 * 1024,
		temperature: 0,
		modelName: 'llama3:8b-instruct-q6_K',
		prompt: '',
		toolNames: [],
		webSearchCategories: [],
		session: 'new',
		amountOfResults: 4,
		minResultScore: 0.5,
		amountOfWebsites: 10,
		chunkSize: 300,
		chunkOverlap: 100,
		systemMessage: `1. Format your "final answer" in markdown.
2. You may use tools more than once.
3. Answer in the same language as the question.`
	};

	let clientValues: ClientSettings = defaultClientValues;

	function newChat() {
		goto('/chat/new');
		if (eventSource !== null) {
			eventSource.close();
		}
		eventSource = null;
		currentLogs = [];
		searchSources = [];
		clientValues.session = 'new';
	}

	function stopChat() {
		if (eventSource !== null) {
			eventSource.close();
		}
		eventSource = null;
	}

	function setSessionToChatId(chatid: string) {
		clientValues.session = chatid;
		console.log('changed session to', chatid);
	}

	setSessionToChatId($page.params.chatid);

	// Establish a connection to the server-sent events endpoint
	function sendPrompt() {
		searchSources = [];

		let clientSettingsJsonString = JSON.stringify(clientValues);

		let url = '/api/stream?settings=' + encodeURIComponent(clientSettingsJsonString);
		// console.log(url);
		// console.log(clientSettingsJsonString);

		sendMode = false;
		clientValues.prompt = '';
		currentLogs = currentLogs;
		eventSource = new EventSource(url);
		eventSource.onmessage = (event: MessageEvent) => {
			let log: LogElement = JSON.parse(event.data);
			if (log.stepType == StepType.HandleNewSession) {
				console.log('new session', log.session);
				clientValues.session = log.session;
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
				let currentLogIndex = currentLogs.length;
				setTimeout(() => {
					for (let i = currentLogIndex; i < currentLogs.length; i++) {
						if (currentLogs[i].stream) {
							return;
						}
					}
					let newLogElement: LogElement = {
						message: `Ollama is currently loading the model. This might take a few seconds.`,
						stepType: StepType.HandleOllamaModelLoadMessage
					};
					currentLogs.push(newLogElement);
					currentLogs = currentLogs;
				}, 8000);
			}

			if (log.message) {
				if (log.stepType == StepType.HandleSourceAdded) {
					searchSources.push(log.source);
					searchSources = searchSources;
				}
				log.message = log.message.replaceAll('<|im_end|>', '').replaceAll('<|end_of_turn|>', '');
				if (log.stream) {
					if (lastLogitemWasStream) {
						currentLogs[currentLogs.length - 1].message += log.message;
					} else {
						currentLogs.push(log);
					}
					lastLogitemWasStream = true;
				} else {
					lastLogitemWasStream = false;
					currentLogs.push(log);
				}
			}

			currentLogs = currentLogs;
		};

		eventSource.onerror = (error: Event) => {
			console.error('EventSource failed:', error);
			eventSource?.close();
			eventSource = null;
			sendMode = true;
		};
	}
	onDestroy(() => {
		eventSource?.close();
	});

	let showSidebar = true;
	onMount(() => {
		// if screen width is more than 640px, show sidebar
		if (window.innerWidth > 640) {
			showSidebar = true;
		} else {
			showSidebar = false;
		}
	});

	// handle the whole darkmode thing
	let isDarkMode = false;
	$: changeDarkMode(isDarkMode);
	onMount(() => {
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
	function scollToElement() {
		if (scrollContainer === undefined) {
			return;
		}
		console.log('scrolling to element');
		scrollContainer.scrollIntoView({ behavior: 'smooth', block: 'end' });
	}
	onMount(() => {
		scollToElement();
	});

	$: currentLogs, scollToElement();
</script>

<svelte:head>
	<link rel="preconnect" href="https://fonts.googleapis.com" />
	<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin="true" />
	<link
		href="https://fonts.googleapis.com/css2?family=Vollkorn:ital,wght@0,400..900;1,400..900&display=swap"
		rel="stylesheet"
	/>
	<title>Chat</title>
</svelte:head>

<div class="flex flex-col h-svh w-svw text-stone-800">
	<div class="w-full bg-stone-200 flex px-2 pt-2 justify-between border border-stone-300">
		<div class="flex gap-2">
			<ToggleSidebarButton bind:showSidebar />
			<NewChatButton action={newChat} />
			<!-- {#if showSidebar} -->
			<!-- 	<NewChatButton action={newChat} /> -->
			<!-- {/if} -->
		</div>
		<div class="flex gap-2">
			<ShowLogsButton bind:showLogs />
			<ToggleDarkmodeButton bind:isDarkMode />
		</div>
	</div>
	<div class="flex flex-grow">
		<div>
			{#if showSidebar}
				<div
					in:slide={{ duration: 200, axis: 'x' }}
					out:slide={{ duration: 200, axis: 'x' }}
					class="w-56 bg-stone-300 shadow-inner p-2 overflow-scroll h-full border-stone-300 border-r"
				>
					{#await data.item?.fetchChats}
						<!-- <div> -->
						<!-- 	<ChatList loading={true} /> -->
						<!-- </div> -->
					{:then chatListItems}
						<div in:fade={{ duration: 100 }}>
							{#if chatListItems}
								<ChatList {chatListItems} bind:session={clientValues.session} />
							{/if}
						</div>
					{:catch error}
						<p style="color: red">{error.message}</p>
					{/await}
				</div>
			{/if}
		</div>
		<div class="flex flex-col h-full flex-grow overflow-hidden">
			<div class="flex-grow flex justify-center h-96 overflow-scroll p-4">
				{#await data.item?.fetchHistory}
					<div class="flex flex-col gap-2">
						<LoadingMessage />
						<LoadingMessage />
						<LoadingMessage />
					</div>
				{:then chatHistoryItems}
					<div>
						{#if chatHistoryItems}
							{#each chatHistoryItems as logElement}
								<LogItem bind:showLogs bind:logElement />
							{/each}
							<!-- {:else} -->
							<!-- 	<p>No chat history available</p> -->
						{/if}
						{#each currentLogs as logElement}
							<div bind:this={scrollContainer}>
								<LogItem bind:showLogs bind:logElement />
							</div>
						{/each}
					</div>
				{:catch error}
					<p class="text-red-600">{error.message}</p>
					<!-- {#each currentLogs as logElement} -->
					<!-- 	<div bind:this={scrollContainer}> -->
					<!-- 		<LogItem showLogs={false} bind:logElement /> -->
					<!-- 	</div> -->
					<!-- {/each} -->
				{/await}
			</div>
			<div class="flex justify-center p-4">
				<!-- <div class="w-72 rounded-lg bg-stone-50 border-stone-400 px-4 py-3 shadow"> -->
				<BottomBar
					bind:sendMode
					bind:eventSource
					bind:prompt={clientValues.prompt}
					{sendPrompt}
					{newChat}
					{stopChat}
				></BottomBar>
				<!-- </div> -->
			</div>
		</div>
	</div>
</div>

<style lang="postcss">
	:global(html) {
		background-color: theme(colors.stone.200);
		font-family: 'Vollkorn', serif;
	}
</style>
