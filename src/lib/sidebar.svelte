<script lang="ts">
	import { draw, fade, slide } from 'svelte/transition';
	import type { ChatListItem, Source } from './types/types';
	import ChatList from './chatList.svelte';
	import Sources from './sources.svelte';
	export let rightBarMode: string;
	export let searchSources: Source[];
	export let session: string;
	export let loadHistory: Function;

	export let chatlistItems: Promise<ChatListItem[]>;
</script>

<div
	in:slide={{ duration: 200, axis: 'x' }}
	out:slide={{ duration: 200, axis: 'x' }}
	class="bg-neutral-100 shadow-inner p-2 overflow-scroll h-full border-neutral-300 border-r transition-all dark:bg-neutral-900 dark:border-neutral-800"
>
	{#if rightBarMode == 'chats'}
		<div class="w-64">
			<!-- <div -->
			<!-- 	class="flex justify-center p-1 px-2 shadow-inner hover:cursor-pointer truncate rounded bg-neutral-200 dark:bg-neutral-700 transition-all text-sm dark:text-neutral-300" -->
			<!-- > -->
			<!-- 	<svg -->
			<!-- 		xmlns="http://www.w3.org/2000/svg" -->
			<!-- 		fill="none" -->
			<!-- 		viewBox="0 0 24 24" -->
			<!-- 		stroke-width="1.2" -->
			<!-- 		stroke="currentColor" -->
			<!-- 		class="w-6 h-6" -->
			<!-- 	> -->
			<!-- 		<path -->
			<!-- 			in:draw={{ duration: 300 }} -->
			<!-- 			stroke-linecap="round" -->
			<!-- 			stroke-linejoin="round" -->
			<!-- 			d="M12 4.5v15m7.5-7.5h-15" -->
			<!-- 		/> -->
			<!-- 	</svg> -->
			<!-- </div> -->
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
			<Sources {searchSources} />
		</div>
	{/if}
</div>
