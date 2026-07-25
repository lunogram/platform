import { useEffect, useState } from "react"

/**
 * Tracks the console's light/dark appearance.
 *
 * The console signals dark mode with a `dark` class on `<html>` (see the
 * `@custom-variant dark` rule in index.css). Nothing sets that class today —
 * PreferencesContext pins `mode` to "light" and its theme effect is commented
 * out — so this currently always reports "light". Reading the class rather
 * than the preference means the editor follows automatically once dark mode is
 * wired up, with no further change here.
 *
 * The same detect-plus-observe pattern is used by `components/ui/code-editor`
 * and `components/ui/map`; worth extracting to a shared hook if a fourth
 * consumer appears.
 */
export function useConsoleTheme(): "light" | "dark" {
    const [theme, setTheme] = useState<"light" | "dark">(() =>
        document.documentElement.classList.contains("dark") ? "dark" : "light",
    )

    useEffect(() => {
        const observer = new MutationObserver(() => {
            setTheme(document.documentElement.classList.contains("dark") ? "dark" : "light")
        })
        observer.observe(document.documentElement, {
            attributes: true,
            attributeFilter: ["class"],
        })
        return () => observer.disconnect()
    }, [])

    return theme
}
