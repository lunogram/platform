import type { StorybookConfig } from "@storybook/react-vite"

// Storybook reuses the app's vite.config.ts, so the `@/` alias, Tailwind, the
// enterprise import stubs, and the `__ENTERPRISE__` define match `pnpm build`.
const config: StorybookConfig = {
    stories: ["../stories/**/*.stories.tsx"],
    // Serves the stylesheets in preview-head.html, as the app's index.html does.
    staticDirs: ["../public"],
    framework: {
        name: "@storybook/react-vite",
        options: {},
    },
    core: {
        disableTelemetry: true,
    },
    async viteFinal(config) {
        // Stories render deterministic fixtures; never proxy to a running API.
        return {
            ...config,
            server: { ...config.server, proxy: {} },
        }
    },
}

export default config
