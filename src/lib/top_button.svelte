<script lang="ts">
	import { fade } from 'svelte/transition';
</script>

<div
	in:fade={{ duration: 300 }}
	out:fade={{ duration: 300 }}
	class="hover:bg-stone-100 hover:shadow text-stone-600 hover:text-stone-700 rounded-2xl hover:cursor-pointer transition-all dark:text-stone-500 dark:hover:text-stone-300 dark:hover:bg-stone-900"
>
	<slot />
</div>
