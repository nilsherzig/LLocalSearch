<script lang="ts">
	export let value: number;
	export let descriptionBefore: string = '';
	export let descriptionAfter: string = '';
	export let maxValue: number;
	export let minValue: number;
	export let stepSize: number;
	let inputValue = value;
	let showRange = false;

	function validateValue(iv: number) {
		if (
			iv < minValue ||
			iv > maxValue ||
			isNaN(iv) ||
			iv == null ||
			iv == undefined ||
			iv == Infinity ||
			iv == -Infinity
		) {
			showRange = true;
			return;
		}

		showRange = false;
		inputValue = iv;
		value = iv;
	}

	// if the value changes somewhere else
	// (i.e. loaded from local storage or settings reset)
	$: validateValue(value);

	// if the user changes as value
	$: validateValue(inputValue);

	let inputElement: HTMLInputElement;
</script>

<div class="flex flex-col">
	<p>
		{descriptionBefore}
		<input
			class="rounded w-20 dark:bg-stone-800 px-1 py-0.5 outline-none bg-stone-200 font-bold
        [appearance:textfield] [&::-webkit-outer-spin-button]:appearance-none [&::-webkit-inner-spin-button]:appearance-none
        "
			type="number"
			bind:this={inputElement}
			bind:value={inputValue}
			min={minValue}
			max={maxValue}
			step={stepSize}
		/>
		{descriptionAfter}
	</p>
	<p>
		{#if showRange}
			<span class="text-red-500">Value must be between {minValue} and {maxValue}</span>
		{/if}
	</p>
	<!-- <input -->
	<!-- 	class="rounded p-1" -->
	<!-- 	type="range" -->
	<!-- 	bind:value={bufferValue} -->
	<!-- 	on:input={adjustInputWidth} -->
	<!-- 	min={minValue} -->
	<!-- 	max={maxValue} -->
	<!-- 	step={stepSize} -->
	<!-- /> -->
</div>
