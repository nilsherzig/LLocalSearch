<script lang="ts">
	export let value: number;
	export let descriptionBefore: string = '';
	export let descriptionAfter: string = '';
	export let maxValue: number;
	export let minValue: number;
	export let stepSize: number;
	let bufferValue = value;

	function validateValue(bv: number) {
		if (bv == null) {
			bv = minValue;
		}

		if (bv < minValue) {
			// bv = minValue;
			return;
		}

		if (bv > maxValue) {
			// bv = maxValue;
			return;
		}

		bufferValue = bv;
		value = bv;
	}

	$: bufferValue = value;
	$: validateValue(bufferValue);

	let inputElement: HTMLInputElement;

	// Function to adjust the input width
	// const adjustInputWidth = () => {
	// 	if (typeof window === 'undefined') return;
	// 	console.log('adjusting input width');
	// 	const tempElement = document.createElement('span');
	// 	tempElement.style.visibility = 'hidden';
	// 	tempElement.style.position = 'absolute';
	// 	tempElement.style.height = 'auto';
	// 	tempElement.style.width = 'auto';
	// 	tempElement.style.whiteSpace = 'pre';
	// 	// Apply the same font styling as your input for accurate measurement
	// 	tempElement.style.font = getComputedStyle(inputElement).font;
	// 	document.body.appendChild(tempElement);
	//
	// 	// Update the content of the temporary element with the input's value
	// 	tempElement.textContent = inputElement.value || inputElement.placeholder;
	//
	// 	// Adjust the input width based on the temporary element's width
	// 	inputElement.style.width = `${tempElement.offsetWidth + 2}px`; // +2 for border or padding adjustments
	//
	// 	// Clean up
	// 	document.body.removeChild(tempElement);
	// };

	// onMount(() => {
	// 	adjustInputWidth(); // Adjust width on initial load
	// });
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
			bind:value={bufferValue}
			min={minValue}
			max={maxValue}
			step={stepSize}
		/>
		{descriptionAfter}
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
