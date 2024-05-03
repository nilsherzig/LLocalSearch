<script lang="ts">
	import { onMount } from 'svelte';
	import ChatButton from './chat_button.svelte';
	export let prompt: string;
	export let sendPrompt: () => void;
	export let newChat: () => void;
	export let sendMode = true;
	let textArea: HTMLTextAreaElement;
	export let eventSource: EventSource | null;
	export let stopChat: () => void;

	function autoResize() {
		textArea.style.height = 'auto'; // Reset height to recalibrate
		textArea.style.height = textArea.scrollHeight + 'px'; // Set new height based on scroll height
	}

	onMount(() => {
		autoResize(); // Initial resize to adjust for any default text
		window.addEventListener('resize', autoResize);

		return () => {
			// Cleanup
			window.removeEventListener('resize', autoResize);
		};
	});
	function handleKeyDown(event: KeyboardEvent) {
		if (sendMode) {
			if (event.key === 'Enter') {
				if (!event.shiftKey && !event.altKey && !event.ctrlKey && !event.metaKey) {
					event.preventDefault(); // Prevents the default action of the enter key (e.g., submitting a form)
					sendPrompt();
				}
			}
		}
		autoResize(); // Initial resize to adjust for any default text
	}
</script>

<div class="w-full flex items-center justify-around flex-col px-4">
	<div class="max-w-prose w-full md:px-6">
		<form
			class="mb-2 max-w-prose flex gap-2 w-full align-middle bg-neutral-100 shadow items-center border-neutral-300 border p-1 rounded-lg focus-within:shadow-lg focus-within:border-neutral-400 focus-within:dark:border-neutral-500 transition-all dark:bg-neutral-800 dark:border-neutral-700"
		>
			<textarea
				class="resize-none outline-none rounded bg-neutral-100 py-1 px-2 text-neutral-700 flex-grow dark:bg-neutral-800 dark:text-neutral-100 transition-all"
				bind:value={prompt}
				bind:this={textArea}
				on:input={autoResize}
				on:keydown={handleKeyDown}
				rows="1"
				placeholder="Ask me something about llamas..."
				autofocus
			></textarea>
			<ChatButton {prompt} bind:sendMode bind:eventSource {sendPrompt} {newChat} {stopChat} />
		</form>
	</div>
	<!-- <span class="text-neutral-300 text-xs pb-1" -->
	<!-- 	>Its impossible for LLocalSearch to make mistakes, dont question anything</span -->
	<!-- > -->
</div>
