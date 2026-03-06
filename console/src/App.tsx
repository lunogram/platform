import { useMemo } from "react"
import { RouterProvider } from "react-router"
import { PreferencesProvider } from "./contexts/PreferencesContext"
import type { RouterProps } from "./views/router"
import { createRouter } from "./views/router"
import { Toaster } from "@/components/ui/sonner"

export default function App(props: RouterProps) {
    const router = useMemo(() => createRouter(props), [props])

    return (
        <PreferencesProvider>
            <RouterProvider router={router} />
            <Toaster />
        </PreferencesProvider>
    )
}
