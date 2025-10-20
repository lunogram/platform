import { defineConfig, loadEnv } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig(({ mode }) => {
    // Load env file based on `mode` (e.g. .env.development or .env.production)
    const env = loadEnv(mode, process.cwd(), '')

    return {
        base: '/',
        plugins: [react()],
        server: {
            proxy: {
                '/api': {
                    target: env.VITE_PROXY_URL,
                    changeOrigin: true,
                },
                '/unsubscribe': {
                    target: env.VITE_PROXY_URL,
                    changeOrigin: true,
                    rewrite: path => path.replace(/^\/unsubscribe/, '/api/unsubscribe'),
                },
            },
        },
        test: {
            globals: true,
            environment: 'jsdom',
            setupFiles: './src/setupTests.ts',
            css: true,
            reporters: ['verbose'],
            coverage: {
                reporter: ['text', 'json', 'html'],
                include: ['src/**/*'],
                exclude: [],
            },
        },
        resolve: {
            alias: {
                '@': '/src',
            }
        }
    }
})