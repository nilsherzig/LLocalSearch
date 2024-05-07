import { handler } from './build/handler.js';
import express from 'express';
import { createProxyMiddleware } from 'http-proxy-middleware';

const app = express();

const apiProxyOptions = {
	target: 'http://backend:8080', // URL of your backend
	changeOrigin: true,
	pathRewrite: {
		'^/api': '' // rewrite path
	}
};

// Apply proxy to /api
app.use('/api', createProxyMiddleware(apiProxyOptions));

// let SvelteKit handle everything else, including serving prerendered pages and static assets
app.use(handler);

app.listen(3000, () => {
	console.log('frontend listening on port 3000');
});
