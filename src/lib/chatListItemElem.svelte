<script lang="ts">
	import type { ChatListItem } from './types/types';
	export let item: ChatListItem;
	export let session: string;
	export let loadHistory: Function;
</script>

<button on:click={() => loadHistory(item.sessionid)}>
	{#if item.sessionid === session}
		<div
			class="text-left p-1 px-2 shadow-inner hover:cursor-pointer truncate rounded bg-stone-200 dark:bg-stone-700 transition-all text-sm dark:text-stone-300"
		>
			<span>{item.title.replace('title: ', '')}</span>
		</div>
	{:else}
		<div
			class="text-left p-1 px-2 hover:cursor-pointer hover:shadow-inner truncate rounded bg-stone-100 hover:bg-stone-200 dark:bg-stone-900 hover:dark:bg-stone-700 dark:text-stone-300 transition-all text-sm"
		>
			<span>{item.title.replace('title: ', '')}</span>
		</div>
	{/if}
</button>
