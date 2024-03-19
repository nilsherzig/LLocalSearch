<script lang="ts">
	import { onMount } from 'svelte';
	export let prompt: string;
	export let toggleChat: () => void;
	let textArea: HTMLTextAreaElement;

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
		if (event.key === 'Enter') {
			event.preventDefault(); // Prevents the default action of the enter key (e.g., submitting a form)
			toggleChat();
			autoResize(); // Initial resize to adjust for any default text
		}
	}
</script>

<div class="w-full flex items-center flex-col px-4">
	<form
		class="m-2 mx-4 flex max-w-prose gap-2 shadow w-full align-middle bg-stone-50 items-center border-stone-300 border-2 p-1 rounded-lg focus-within:shadow-lg focus-within:border-stone-400 transition-all"
	>
		<textarea
			class="resize-none outline-none rounded bg-stone-50 py-1 px-2 text-zinc-950 flex-grow"
			bind:value={prompt}
			bind:this={textArea}
			on:input={autoResize}
			on:keydown={handleKeyDown}
			rows="1"
			placeholder="Ask me something about llamas..."
		></textarea>
		<button
			class="bg-stone-200 text-stone-500 hover:text-stone-700 hover:shadow hover:cursor-pointer hover:bg-stone-300 m-1 p-1 transition-all rounded-xl w-8 h-8"
			type="submit"
			on:click={toggleChat}
		>
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
					d="M16.023 9.348h4.992v-.001M2.985 19.644v-4.992m0 0h4.992m-4.993 0 3.181 3.183a8.25 8.25 0 0 0 13.803-3.7M4.031 9.865a8.25 8.25 0 0 1 13.803-3.7l3.181 3.182m0-4.991v4.99"
				/>
			</svg>
		</button>
	</form>
</div>
