<script lang="ts">
	import { goto } from '$app/navigation';
	import ChatList from '$lib/chatList.svelte';
	import { type ChatListItem } from '$lib/types/types';
	async function fetchChats(): Promise<ChatListItem[]> {
		const response = await fetch('/api/chats');
		const data = await response.json();
		if (response.ok) {
			console.log(data);
			return data;
		} else {
			throw new Error(data.message);
		}
	}
	let chatListItems = fetchChats();
</script>

<div class="flex gap-4">
	<div class="bg-red-100 p-2">
		{#await chatListItems}
			<p>Loading...</p>
		{:then chatListItems}
			<ChatList {chatListItems} />
		{:catch error}
			<p style="color: red">{error.message}</p>
		{/await}
	</div>
</div>
