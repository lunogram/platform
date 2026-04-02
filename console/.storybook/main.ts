import type { StorybookConfig } from "@storybook/react-vite"
import { mergeConfig } from "vite"
import tailwindcss from "@tailwindcss/vite"
import path from "path"
import { fileURLToPath } from "url"

const __dirname = path.dirname(fileURLToPath(import.meta.url))

const config: StorybookConfig = {
    stories: ["../src/components/**/*.stories.@(ts|tsx)"],
    addons: [
        "@chromatic-com/storybook",
        "@storybook/addon-a11y",
        "@storybook/addon-docs",
        "@storybook/addon-onboarding",
    ],
    framework: "@storybook/react-vite",
    viteFinal: async (config) => {
        return mergeConfig(config, {
            plugins: [tailwindcss()],
            resolve: {
                alias: { "@": path.resolve(__dirname, "../src") },
            },
        })
    },
}

export default config
