import { useMemo } from "react"
import { RouterProvider } from "react-router"
import { QueryClientProvider } from "@tanstack/react-query"
import { ReactQueryDevtools } from "@tanstack/react-query-devtools"
import { PreferencesProvider } from "./contexts/PreferencesContext"
import { queryClient } from "./lib/query-client"
import type { RouterProps } from "./views/router"
import { createRouter } from "./views/router"
import { Toaster } from "@/components/ui/sonner"

export default function App(props: RouterProps) {
    const router = useMemo(() => createRouter(props), [props])

    return (
        <QueryClientProvider client={queryClient}>
            <PreferencesProvider>
                <RouterProvider router={router} />
                <Toaster />
            </PreferencesProvider>
            {import.meta.env.DEV && <ReactQueryDevtools initialIsOpen={false} />}
        </QueryClientProvider>
    )
}
