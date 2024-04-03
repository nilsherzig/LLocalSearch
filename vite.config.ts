import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

export default defineConfig({
    plugins: [sveltekit()],
    server: {
        proxy: {
            '/api': {
                target: 'http://backend-dev:8080',
                changeOrigin: true,
                rewrite: path => path.replace(/^\/api/, '/') // proxy request to go backend
            }
        }
    }
});
