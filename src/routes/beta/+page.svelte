<script lang="ts">
	import { StepType, type Source, type LogElement } from '$lib/types/types';
	import LogNode from '$lib/log_node.svelte';

	// Enhanced mock data with 5 levels deep structure

	const sourceExample: Source = {
		name: 'Example Source',
		link: 'https://example.com',
		summary: 'An example source for testing purposes.'
	};

	// Creating log elements with a 5 levels deep structure
	const createLogElement = (
		parent: LogElement | null,
		level: number,
		stepType: StepType,
		message: string
	): LogElement => {
		return {
			children: [],
			close: level === 5, // Close at the last level
			message: message,
			parent: parent,
			source: sourceExample,
			stepLevel: level,
			stepType: stepType,
			stream: level % 2 === 0 // Alternate stream value
		};
	};

	// Root element
	const rootLogElement: LogElement = createLogElement(
		null,
		0,
		StepType.HandleChainStart,
		'root log'
	);
	let inOpenChain = false;
	let inOpenTool = false;
	let chainHead: LogElement;

	let counter = 0;
	// Recursive function to populate children
	const populateChildren = (parent: LogElement, lastLevel: number) => {
		if (counter < 20) {
			counter++;
			// const childStepType = Object.values(StepType)[currentLevel + 1] as StepType; // Sequentially assign step types for simplicity
			// const childStepType = StepType.HandleChainStart;
			let childStepLevel = lastLevel;
			let childStepType: StepType = StepType.HandleChainEnd; // default type
			let childParent = parent;

			if (parent.stepType == StepType.HandleChainStart) {
				let validSteps: StepType[] = [];
				validSteps.push(StepType.HandleToolStart);
				validSteps.push(StepType.HandleAgentAction);
				validSteps.push(StepType.HandleChainEnd);
				childStepType = randomStepType(validSteps);
			}

			if (parent.stepType == StepType.HandleToolStart) {
				let validSteps: StepType[] = [];
				validSteps.push(StepType.HandleToolEnd);
				childStepType = randomStepType(validSteps);
			}

			if (parent.stepType == StepType.HandleAgentAction) {
				let validSteps: StepType[] = [];
				validSteps.push(StepType.HandleToolStart);
				childStepType = randomStepType(validSteps);
			}

			// if (childStepType == StepType.HandleChainEnd && childStepLevel == 1) {
			// 	console.log('exit');
			// 	return;
			// }

			if (
				childStepType === StepType.HandleChainEnd ||
				childStepType == StepType.HandleToolEnd ||
				childStepType == StepType.HandleLlmEnd
			) {
				childParent = parent.parent;
				childStepLevel = lastLevel - 1;
			} else if (
				childStepType === StepType.HandleChainStart ||
				childStepType == StepType.HandleToolStart ||
				childStepType == StepType.HandleAgentAction ||
				childStepType == StepType.HandleLlmStart
			) {
				childStepLevel = lastLevel + 1;
			}

			const child = createLogElement(
				parent,
				lastLevel + 1,
				childStepType,
				randomNumber().toString()
			);
			parent.children.push(child);

			populateChildren(childParent, childStepLevel);
		}
	};

	const randomNumber = () => {
		return Math.floor(Math.random() * 10000);
	};

	const randomStepType = (validSteps: StepType[]) => {
		return validSteps[Math.floor(Math.random() * validSteps.length)];
	};

	// Start populating from the root
	populateChildren(rootLogElement, 0);

	console.log(rootLogElement);
</script>

<div class="text-zinc-200">
	<LogNode logElement={rootLogElement} />
</div>

<style lang="postcss">
	:global(html) {
		background-color: theme(colors.zinc.950);
	}
</style>
