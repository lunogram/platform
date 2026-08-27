import { useMemo } from "react"
import { RouterProvider } from "react-router"
import { PreferencesProvider } from "./contexts/PreferencesContext"
import { useSessionRefresh } from "./hooks/use-session-refresh"
import type { RouterProps } from "./views/router"
import { createRouter } from "./views/router"
import { Toaster } from "@/components/ui/sonner"

export default function App(props: RouterProps) {
    const router = useMemo(() => createRouter(props), [props])

    // The console session has an idle window; without this a tab left open
    // simply stops working part way through the day.
    useSessionRefresh()

    return (
        <PreferencesProvider>
            <RouterProvider router={router} />
            <Toaster />
        </PreferencesProvider>
    )
}
