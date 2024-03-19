<script lang="ts">
	import { StepType, type Source, type LogElement } from '$lib/types/types';
	export let logElement: LogElement;

	let open = true;

	function toggleOpen() {
		open = !open;
	}
	console.log(logElement);
	if (logElement.children === undefined) {
		logElement.children = [];
	}
	let children = logElement.children;
</script>

<!-- <h3 style="padding-left: {indent}px" on:click={toggleOpen}> -->
<!-- 	{name} -->
<!-- 	{open ? '(open)' : '(closed)'} -->
<!-- </h3> -->
<p style="padding-left: {logElement.stepLevel * 2}rem;">
	<button on:click={toggleOpen}>
		<!-- {logElement.message} -->
		{logElement.stepLevel}:
		{logElement.stepType} ->
		{logElement.message}
		{open ? '(open)' : '(closed)'}
	</button>
	{#if logElement.parent}
		Parent: {logElement.parent.message}
	{/if}
</p>

{#if open}
	{#each children as child}
		<svelte:self logElement={child} />
	{/each}
{/if}
