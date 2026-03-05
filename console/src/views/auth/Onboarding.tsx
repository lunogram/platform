import { Outlet } from "react-router"

export default function Onboarding() {
    return (
        <div className="flex min-h-screen flex-col items-center justify-center gap-4 bg-muted/40 p-10">
            <Outlet />
        </div>
    )
}
