<script lang="ts">
	import { fade, draw } from 'svelte/transition';
	import type { ClientSettings } from './types/types';
	import SettingsField from './settings_field.svelte';
	export let showSettings: boolean;
	export let clientSettings: ClientSettings;
	export let models: string[];
	export let defaultClientValues: ClientSettings;
	import { clickOutside } from './clickOutside.js';
</script>

{#if showSettings}
	<div
		in:fade={{ duration: 100 }}
		out:fade={{ duration: 100 }}
		class="fixed backdrop-blur inset-0 z-10 flex flex-col items-center justify-center bg-opacity-20 dark:bg-opacity-80 bg-neutral-950"
	>
		<div
			use:clickOutside
			on:click_outside={() => {
				showSettings = false;
			}}
			class="m-4 max-w-prose bg-neutral-50 p-4 rounded-lg shadow-lg border border-neutral-300 dark:bg-neutral-900 dark:border-neutral-800 dark:text-neutral-200 overflow-scroll z-50"
		>
			<div
				class="flex flex-row justify-between border-b border-b-neutral-300 pb-2 dark:border-b-neutral-700
                text-neutral-600 dark:text-neutral-400
                "
			>
				<p class="font-bold p-1">Settings</p>
				<div>
					<span title="reset settings">
						<button
							class="rounded-lg p-1 dark:bg-neutral-900 bg-neutral-50 hover:cursor-pointer hover:bg-neutral-200 hover:text-neutral-800 dark:hover:text-neutral-300 transition-all dark:hover:bg-neutral-800"
							on:click={() => {
								clientSettings = JSON.parse(JSON.stringify(defaultClientValues)); // holy fuck i hate js
							}}
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
									in:draw={{ duration: 300 }}
									stroke-linecap="round"
									stroke-linejoin="round"
									d="M16.023 9.348h4.992v-.001M2.985 19.644v-4.992m0 0h4.992m-4.993 0 3.181 3.183a8.25 8.25 0 0 0 13.803-3.7M4.031 9.865a8.25 8.25 0 0 1 13.803-3.7l3.181 3.182m0-4.991v4.99"
								/>
							</svg>
						</button>
					</span>
					<span title="close settings">
						<button
							class="rounded-lg p-1 dark:bg-neutral-900 bg-neutral-50 hover:cursor-pointer hover:bg-neutral-200 hover:text-neutral-800 dark:hover:text-neutral-300 transition-all dark:hover:bg-neutral-800"
							on:click={() => {
								showSettings = false;
							}}
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
									in:draw={{ duration: 300 }}
									stroke-linecap="round"
									stroke-linejoin="round"
									d="M6 18 18 6M6 6l12 12"
								/>
							</svg>
						</button>
					</span>
				</div>
			</div>

			<div class="flex flex-col gap-2">
				<p class="font-bold mt-4">LLM</p>
				<div>
					<p>
						The agent chain is using the
						<select
							class="bg-neutral-200 dark:bg-neutral-800 px-1 py-0.5 rounded"
							bind:value={clientSettings.modelName}
						>
							{#if models != undefined}
								{#if models.length === 0}
									<option value="loading">loading...</option>
								{:else}
									{#each models as model}
										<option value={model}>{model}</option>
									{/each}
								{/if}
							{/if}
						</select>
						model.
					</p>
				</div>
				<div>
					<p>The llm uses the following system message:</p>
					<textarea
						class="bg-neutral-200 dark:bg-neutral-800 p-2 rounded w-full"
						rows="5"
						bind:value={clientSettings.systemMessage}
					/>
				</div>
				<SettingsField
					bind:value={clientSettings.temperature}
					maxValue={10}
					minValue={0}
					stepSize={0.1}
					descriptionBefore="The agent will be set to a temperature of"
					descriptionAfter="."
				/>
				<SettingsField
					bind:value={clientSettings.contextSize}
					maxValue={128000}
					minValue={1}
					stepSize={1024}
					descriptionBefore="The agent will have a context size of"
					descriptionAfter="tokens."
				/>
				<SettingsField
					bind:value={clientSettings.maxIterations}
					maxValue={50}
					minValue={1}
					stepSize={1}
					descriptionBefore="The agent will perform a maximum of"
					descriptionAfter="iterations (actions and evaluations) per question."
				/>
				<p class="font-bold mt-4">Vector DB</p>
				<SettingsField
					bind:value={clientSettings.minResultScore}
					maxValue={1}
					minValue={0}
					stepSize={0.1}
					descriptionBefore="DB entries need a minimum score of"
					descriptionAfter="to be returned to the llm."
				/>
				<SettingsField
					bind:value={clientSettings.amountOfResults}
					maxValue={30}
					minValue={1}
					stepSize={1}
					descriptionBefore="A db query will return a maximum of"
					descriptionAfter="results."
				/>
				<p class="font-bold mt-4">Webscraper</p>
				<SettingsField
					bind:value={clientSettings.amountOfWebsites}
					maxValue={30}
					minValue={1}
					stepSize={1}
					descriptionBefore="The website scraper will scrape a maximum of"
					descriptionAfter="websites per query."
				/>
				<p class="font-bold mt-4">Text chunks / splits</p>
				<SettingsField
					bind:value={clientSettings.chunkSize}
					maxValue={1000}
					minValue={10}
					stepSize={1}
					descriptionBefore="Source text will be split into chunks of maximum"
					descriptionAfter="chars. Chunking will try to split text at new lines or paragraphs."
				/>
				<SettingsField
					bind:value={clientSettings.chunkOverlap}
					maxValue={500}
					minValue={1}
					stepSize={1}
					descriptionBefore="The chunks will overlap by"
					descriptionAfter="chars."
				/>
			</div>
		</div>
	</div>
	<!-- <div -->
	<!-- 	class="fixed inset-0 bg-black bg-opacity-20 z-10" -->
	<!-- 	on:click={() => { -->
	<!-- 		showSettings = false; -->
	<!-- 	}} -->
	<!-- ></div> -->
{/if}
