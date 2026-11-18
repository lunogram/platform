import type { Preview } from "@storybook/react-vite"
import i18n from "i18next"
import { initReactI18next } from "react-i18next"
import { MemoryRouter } from "react-router"

import { PreferencesContext } from "@/contexts/PreferencesContext"
import type { Preferences } from "@/types"
import en from "../public/locales/en.json"

import "../src/variables.css"
import "../src/index.css"

// The app loads locales over HTTP (src/i18n.ts). Stories bundle the English
// locale and initialise synchronously so every render shows the real copy.
i18n.use(initReactI18next)
    .init({
        lng: "en",
        fallbackLng: "en",
        resources: { en: { translation: en } },
        initImmediate: false,
        interpolation: { escapeValue: false },
    })
    .catch(() => {})

// Fixed preferences keep formatted dates identical across machines.
const preferences: Preferences = { lang: "en", mode: "light", timeZone: "UTC" }

const preview: Preview = {
    parameters: {
        layout: "padded",
    },
    decorators: [
        (Story) => (
            <PreferencesContext.Provider value={[preferences, () => {}]}>
                <MemoryRouter>
                    <Story />
                </MemoryRouter>
            </PreferencesContext.Provider>
        ),
    ],
}

export default preview
