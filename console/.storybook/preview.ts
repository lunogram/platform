import "../src/index.css"
import type { Preview } from "@storybook/react-vite"

const preview: Preview = {
    parameters: {
        // Controls: smart matchers for color/date fields
        controls: {
            matchers: {
                color: /(background|color)$/i,
                date: /Date$/i,
            },
        },

        // Docs: enable autodocs globally (stories already tagged with "autodocs")
        docs: {},

        // A11y: report violations in the panel without failing CI
        a11y: {
            test: "todo",
        },

        backgrounds: {
            default: "light",
            values: [
                { name: "light", value: "oklch(1 0 0)" },
                { name: "dark", value: "oklch(0.145 0 0)" },
            ],
        },
    },
}

export default preview
