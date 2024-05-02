<script lang="ts">
	import { fade, slide } from 'svelte/transition';
	import type { ChatListItem, Source } from './types/types';
	import ChatList from './chatList.svelte';
	export let rightBarMode: string;
	export let searchSources: Source[];
	export let session: string;
	export let loadHistory: Function;

	export let chatlistItems: Promise<ChatListItem[]>;
</script>

<div
	in:slide={{ duration: 200, axis: 'x' }}
	out:slide={{ duration: 200, axis: 'x' }}
	class="w-fit bg-stone-100 shadow-inner p-2 overflow-scroll h-full border-stone-300 border-r transition-all dark:bg-stone-900 dark:border-stone-800"
>
	{#if rightBarMode == 'chats'}
		<div class="w-64">
			{#await chatlistItems}
				<!-- <div> -->
				<!-- 	<ChatList loading={true} /> -->
				<!-- </div> -->
			{:then chatListItems}
				<div in:fade={{ duration: 100 }}>
					{#if chatListItems}
						<ChatList {chatListItems} bind:session {loadHistory} />
					{/if}
				</div>
			{:catch error}
				<p style="color: red">{error.message}</p>
			{/await}
		</div>
	{:else if rightBarMode == 'sources'}
		<div class="w-64 flex flex-col gap-2 h-full overflow-scroll">
			{#if searchSources.length > 0}
				{#each searchSources as source}
					<div
						class="bg-stone-100 hover:bg-stone-50 hover:cursor-pointer border-2 border-stone-300 p-2 rounded-lg shadow text-sm text-stone-600"
						in:slide
					>
						<span>{source.title}</span>
					</div>
				{/each}
			{/if}
		</div>
	{/if}
</div>
