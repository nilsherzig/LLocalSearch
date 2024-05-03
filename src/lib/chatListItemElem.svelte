<script lang="ts">
	import type { ChatListItem } from './types/types';
	export let item: ChatListItem;
	export let session: string;
	export let loadHistory: Function;
	function removePrefix(str: string) {
		let reg = /.*title: /;
		return str.replace(reg, '');
	}
	let cleanTitle = removePrefix(item.title);
</script>

<button on:click={() => loadHistory(item.sessionid, cleanTitle)}>
	<span title={cleanTitle}>
		{#if item.sessionid === session}
			<div
				class="text-left p-1 px-2 shadow-inner hover:cursor-pointer truncate rounded bg-neutral-200 dark:bg-neutral-800 transition-all text-sm dark:text-neutral-300"
			>
				<span>{cleanTitle}</span>
			</div>
		{:else}
			<div
				class="text-left p-1 px-2 hover:cursor-pointer hover:shadow-inner truncate rounded bg-neutral-100 hover:bg-neutral-200 dark:bg-neutral-900 hover:dark:bg-neutral-800 dark:text-neutral-300 transition-all text-sm"
			>
				<span>{cleanTitle}</span>
			</div>
		{/if}
	</span>
</button>
