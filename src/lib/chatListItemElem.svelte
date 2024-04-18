<script lang="ts">
	import { goto } from '$app/navigation';
	import type { ChatListItem } from './types/types';
	export let item: ChatListItem;
	export let session: string;

	function changeChat(chatid: string) {
		console.log('Changing chat to: ' + chatid);
		goto('/chat/' + chatid);
		session = chatid;
	}
</script>

<button on:click={() => changeChat(item.sessionid)}>
	<div
		class="p-0.5 hover:cursor-pointer h-8 overflow-hidden shadow rounded bg-stone-50 border-2 border-stone-300 hover:bg-stone-100 hover:border-stone-400 transition-all"
	>
		<span>{item.title}</span>
	</div>
</button>
