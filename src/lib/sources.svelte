<script lang="ts">
	import { fade, fly, slide } from 'svelte/transition';
	import type { Source } from './types/types';

	export let searchSources: Source[];
</script>

{#if searchSources.length > 0}
	<div
		class="fixed bg-stone-300 rounded shadow-inner left-4 top-20 bottom-0 overflow-scroll text-sm text-stone-600"
		in:fade
		out:fade
	>
		<div class="flex flex-col-reverse m-4 gap-4">
			{#each searchSources as source}
				<div
					class="rounded h-full w-96 shadow bg-stone-50 flex flex-col p-4 break-after-page"
					in:slide
				>
					<p>{source.summary}</p>
					<div class="flex mt-2">
						<a href={source.link} target="_blank">
							<img
								src="https://www.google.com/s2/favicons?domain={source.link}"
								class="rounded h-4 w-4 inline-block mr-2 filter grayscale"
								alt="favicon"
							/>
						</a>
						<p>{source.engine}</p>
					</div>
				</div>
			{/each}
		</div>
	</div>
{/if}
