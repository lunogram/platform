import { defineConfig, loadEnv } from "vite"
import react from "@vitejs/plugin-react"
import tailwindcss from "@tailwindcss/vite"
import path from "path"

export default defineConfig(({ mode }) => {
    const env = loadEnv(mode, process.cwd(), "")

    const favicon = mode === "development" ? "/favicon-dev.png" : "/favicon.ico"

    const faviconPlugin = {
        name: "favicon",
        transformIndexHtml() {
            return [
                {
                    tag: "link",
                    attrs: { rel: "icon", href: favicon, type: "image/png" },
                    injectTo: "head" as const,
                },
            ]
        },
    }

    return {
        base: "/",
        define: {
            __ENTERPRISE__: JSON.stringify(env.VITE_ENTERPRISE === "true"),
        },
        plugins: [react(), tailwindcss(), faviconPlugin],
        server: {
            proxy: {
                "/api": {
                    target: env.VITE_PROXY_URL,
                    changeOrigin: true,
                },
                "/backoffice": {
                    target: env.VITE_BACKOFFICE_URL || "http://localhost:8081",
                    changeOrigin: true,
                    rewrite: (path) => path.replace(/^\/backoffice/, ""),
                },
                "/courier": {
                    target: env.VITE_COURIER_URL || "http://localhost:8082",
                    changeOrigin: true,
                    rewrite: (path) => path.replace(/^\/courier/, ""),
                },
                "/unsubscribe": {
                    target: env.VITE_PROXY_URL,
                    changeOrigin: true,
                    rewrite: (path) => path.replace(/^\/unsubscribe/, "/api/unsubscribe"),
                },
            },
        },
        test: {
            globals: true,
            environment: "jsdom",
            setupFiles: "./src/setupTests.ts",
            css: true,
            reporters: ["verbose"],
            coverage: {
                reporter: ["text", "json", "html"],
                include: ["src/**/*"],
                exclude: [],
            },
        },
        worker: {
            format: "es",
        },
        resolve: {
            alias: {
                "@": path.resolve(__dirname, "./src"),
            },
        },
    }
})
