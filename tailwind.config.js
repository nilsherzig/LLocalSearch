/** @type {import('tailwindcss').Config} */
export default {
	content: ['./src/**/*.{html,js,svelte,ts}'],
	darkMode: 'selector',
	theme: {
		extend: {
			transitionProperty: {
				width: 'width',
				height: 'height'
			}
		}
	},
	plugins: [require('@tailwindcss/typography')]
};
