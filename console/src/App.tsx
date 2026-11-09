import { useMemo } from "react"
import { QueryClientProvider } from "@tanstack/react-query"
import { RouterProvider } from "react-router"
import { queryClient } from "./lib/query"
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
        <QueryClientProvider client={queryClient}>
            <PreferencesProvider>
                <RouterProvider router={router} />
                <Toaster />
            </PreferencesProvider>
        </QueryClientProvider>
    )
}
