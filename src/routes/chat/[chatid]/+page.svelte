<script lang="ts">
	import { slide } from 'svelte/transition';
	import { onDestroy, onMount } from 'svelte';
	import ToggleSidebarButton from '$lib/toggle_sidebar_button.svelte';
	import LoadingMessage from '$lib/loading_message.svelte';
	import {
		type LogElement,
		type Source,
		type ClientSettings,
		StepType,
		type ChatListItem
	} from '$lib/types/types';
	import LogItem from '$lib/log_item.svelte';
	import { changeDarkMode } from './handle_darkmode';
	import { fetchChats, fetchHistory } from './load_functions';
	import BottomBar from '$lib/bottom_bar.svelte';
	import { page } from '$app/stores';
	import NewChatButton from '$lib/new_chat_button.svelte';
	import ShowLogsButton from '$lib/show_logs_button.svelte';
	import ToggleDarkmodeButton from '$lib/toggle_darkmode_button.svelte';
	import SettingsWindow from '$lib/settings_window.svelte';
	import SidebarSourcesToggle from '$lib/sidebar_sources_toggle.svelte';
	import SidebarHistoryToggle from '$lib/sidebar_history_toggle.svelte';
	import Sidebar from '$lib/sidebar.svelte';
	import ToggleSettingsButton from '$lib/toggle_settings_button.svelte';
	import { pushState, replaceState } from '$app/navigation';
	let innerWidth = 0;
	let innerHeight = 0;

	// let chatHistory: Promise<LogElement[]>;
	let chatLoadID = $page.params.chatid;
	let pageTitle = 'LLocalSearch';
	function loadHistory(id: string, title: string) {
		console.log('loading history', id);
		if (id === '') {
			console.log('id is empty');
			return;
		}
		if (id === undefined) {
			console.log('id is undefined');
			return;
		}
		if (eventSource !== null) {
			eventSource.close();
		}
		eventSource = null;
		searchSources = [];
		currentLogs = [{}];
		clientValues.session = id;
		// chatHistory = fetchHistory(id);
		// console.log(`loading history ${id}`);
		// chatHistory.then((history) => {
		// 	console.log(history);
		// });
		chatLoadID = id;
		window.history.replaceState(history.state, '', `/chat/${id}`);
		if (title) {
			pageTitle = title;
		}
		if (userHasSmallWindow) {
			showSidebar = false;
		}
		// $page.state.chatid = id;
		// pushState(`/chat/${id}`, $page.state.chatid);
	}
	onMount(() => {
		if ($page.params.chatid) {
			loadHistory($page.params.chatid, 'LLocalSearch');
		}
	});
	// $: loadHistory($page.params.chatid);
	// $: loadHistory($page.state.chatid);
	// $: console.log('page state', $page.state.chatid);

	export let showLogs = false;
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
		// modelName: 'llama3:8b-instruct-q6_K',
		// modelName: 'adrienbrault/nous-hermes2pro:Q8_0',
		modelName: 'adrienbrault/nous-hermes2pro-llama3-8b:q6_K',
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
		loadHistory('new', 'new chat');
	}

	function stopChat() {
		if (eventSource !== null) {
			eventSource.close();
		}
		eventSource = null;
	}

	// Establish a connection to the server-sent events endpoint
	function sendPrompt() {
		lastLogitemWasStream = false;
		let switchSession: string;
		console.log(`sending prompt ${clientValues.prompt} to session ${clientValues.session}`);
		searchSources = [];

		let clientSettingsJsonString = JSON.stringify(clientValues);

		let url = '/api/stream?settings=' + encodeURIComponent(clientSettingsJsonString);

		sendMode = false;
		clientValues.prompt = '';
		eventSource = new EventSource(url);
		eventSource.onmessage = (event: MessageEvent) => {
			let log: LogElement = JSON.parse(event.data);
			if (log.stepType == StepType.HandleNewSession) {
				console.log('new session', log.session);
				clientValues.session = log.session;
				switchSession = log.session;
				return;
			}
			if (log.close) {
				eventSource?.close();
				eventSource = null;
				console.log('closing event source');
				sendMode = true;
				chatlistItems = fetchChats(); // update chat list, since title has changed
				if (switchSession) {
					let cleanTitle = log.message.replace(/.*title: /, '');
					loadHistory(switchSession, cleanTitle);
				}
				return;
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
	let rightBarMode = 'chats';
	let userHasSmallWindow = false;

	onMount(() => {
		// if screen width is more than 640px, show sidebar by default
		if (window.innerWidth > 640) {
			showSidebar = true;
		} else {
			showSidebar = false;
		}
	});
	$: innerWidth > 640 ? (userHasSmallWindow = false) : (userHasSmallWindow = true);
	$: showSidebar = !userHasSmallWindow;

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
		scrollContainer.scrollIntoView({ behavior: 'smooth', block: 'end' });
	}
	onMount(() => {
		scollToElement();
		fetch('/api/models')
			.then((response) => response.json())
			.then((data) => {
				models = data;
			});
	});
	$: currentLogs, scollToElement();

	let models: string[];
	let showSettings = false;

	let chatlistItems: Promise<ChatListItem[]>;
	onMount(() => {
		chatlistItems = fetchChats();
	});
</script>

<svelte:head>
	<link rel="preconnect" href="https://fonts.googleapis.com" />
	<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin="true" />
	<link
		href="https://fonts.googleapis.com/css2?family=Vollkorn:ital,wght@0,400..900;1,400..900&display=swap"
		rel="stylesheet"
	/>
	<title>{pageTitle}</title>
</svelte:head>
<svelte:window bind:innerWidth bind:innerHeight />

<SettingsWindow
	bind:clientSettings={clientValues}
	{models}
	{defaultClientValues}
	bind:showSettings
/>
<div class="flex flex-col h-svh w-svw text-neutral-800">
	<div
		class="w-full bg-neutral-100 flex px-2 pt-2 justify-between border-b border-neutral-300 dark:bg-neutral-900 dark:border-neutral-700"
	>
		<div class="flex gap-2">
			{#if userHasSmallWindow}
				<div class="mr-4">
					<ToggleSidebarButton bind:showSidebar />
				</div>
			{/if}
			{#if showSidebar}
				<div class="flex gap-2" in:slide={{ duration: 200, axis: 'x' }}>
					<SidebarSourcesToggle bind:rightBarMode />
					<SidebarHistoryToggle bind:rightBarMode />
				</div>
			{/if}
		</div>
		<div class="flex gap-2">
			<ShowLogsButton bind:showLogs />
			<ToggleDarkmodeButton bind:isDarkMode />
			<ToggleSettingsButton bind:showSettings />
		</div>
	</div>
	<div class="flex flex-grow">
		{#if showSidebar}
			<div class="w-fit h-full">
				<Sidebar
					bind:rightBarMode
					bind:searchSources
					bind:chatlistItems
					bind:session={clientValues.session}
					{loadHistory}
				/>
			</div>
		{/if}
		<div class="flex flex-col h-full flex-grow overflow-hidden">
			<div class="flex-grow flex justify-center h-96 overflow-y-scroll overflow-x-hidden p-4">
				{#await fetchHistory(chatLoadID)}
					<div class="flex flex-col gap-2 max-w-prose w-full">
						<LoadingMessage />
						<LoadingMessage />
						<LoadingMessage />
					</div>
				{:then chatHistoryItems}
					<div class="w-full max-w-prose">
						{#if chatHistoryItems}
							{#each chatHistoryItems as logElement}
								<LogItem bind:showLogs bind:logElement />
							{/each}
						{/if}
						{#each currentLogs as logElement}
							<div bind:this={scrollContainer}>
								<LogItem bind:showLogs bind:logElement />
							</div>
						{/each}
					</div>
				{:catch error}
					<p class="text-red-600">{error.message}</p>
				{/await}
			</div>
			<div class="justify-center pt-2">
				<BottomBar
					bind:sendMode
					bind:eventSource
					bind:prompt={clientValues.prompt}
					{sendPrompt}
					{newChat}
					{stopChat}
				></BottomBar>
			</div>
		</div>
	</div>
</div>

<style lang="postcss">
	:global(html) {
		background-color: theme(colors.neutral.200);
		font-family: 'Vollkorn', serif;
	}
	:global(html.dark) {
		background-color: theme(colors.neutral.950);
		font-family: 'Vollkorn', serif;
	}
</style>
