<script lang="ts">
	import { fade, fly, slide } from 'svelte/transition';
	import type { Source } from './types/types';

	export let searchSources: Source[];
</script>

{#if searchSources.length > 0}
	<div class="flex flex-col-reverse gap-2">
		{#each searchSources as source}
			<a href={source.link} target="_blank">
				<div
					class="rounded h-full shadow-inner bg-neutral-100 flex flex-col p-4 break-after-page text-xs"
					in:slide
				>
					<p>{source.summary}</p>
					<div class="flex mt-2">
						<img
							src="https://www.google.com/s2/favicons?domain={source.link}"
							class="rounded h-4 w-4 inline-block mr-2 filter grayscale"
							alt="favicon"
						/>
						<p>{source.engine}</p>
					</div>
				</div>
			</a>
		{/each}
	</div>
{/if}
