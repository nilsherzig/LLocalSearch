<script lang="ts">
	import { onMount } from 'svelte';
	export let prompt: string;
	export let sendPrompt: () => void;
	export let resetChat: () => void;
	export let sendMode = true;
	let textArea: HTMLTextAreaElement;
	import ChatButton from './chat_button.svelte';

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
				event.preventDefault(); // Prevents the default action of the enter key (e.g., submitting a form)
				sendPrompt();
			}
		}
		autoResize(); // Initial resize to adjust for any default text
	}
</script>

<div class="w-full flex items-center justify-around flex-col px-4">
	<div class="max-w-prose w-full md:px-6">
		<form
			class="mb-2 max-w-prose flex gap-2 shadow w-full align-middle bg-stone-50 items-center border-stone-300 border-2 p-1 rounded-lg focus-within:shadow-lg focus-within:border-stone-400 transition-all dark:bg-stone-800 dark:border-stone-700"
		>
			<textarea
				class="resize-none outline-none rounded bg-stone-50 py-1 px-2 text-stone-700 flex-grow dark:bg-stone-800 dark:text-stone-100 transition-all text-md"
				bind:value={prompt}
				bind:this={textArea}
				on:input={autoResize}
				on:keydown={handleKeyDown}
				rows="1"
				placeholder="Ask me something about llamas..."
				autofocus
			></textarea>
			<ChatButton {prompt} bind:sendMode {sendPrompt} {resetChat} />
		</form>
	</div>
</div>
