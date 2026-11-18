import type { Meta, StoryObj } from "@storybook/react-vite"
import { AlertCircle, Info } from "lucide-react"
import { useEffect } from "react"
import { toast } from "sonner"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { Toaster } from "@/components/ui/sonner"

const meta: Meta = {
    title: "UI/Feedback",
}

export default meta
type Story = StoryObj

export const Alerts: Story = {
    render: () => (
        <div className="flex max-w-xl flex-col gap-4">
            <Alert>
                <AlertTitle>Setup</AlertTitle>
                <AlertDescription>
                    Campaigns depend on message integrations to be able to send.
                </AlertDescription>
            </Alert>
            <Alert>
                <Info className="h-4 w-4" />
                <AlertTitle>Heads up</AlertTitle>
                <AlertDescription>Changes apply to new journey entrances only.</AlertDescription>
            </Alert>
            <Alert variant="destructive">
                <AlertCircle className="h-4 w-4" />
                <AlertTitle>Failed to load campaigns</AlertTitle>
                <AlertDescription>The server returned an error. Try again later.</AlertDescription>
            </Alert>
        </div>
    ),
}

export const Loading: Story = {
    render: () => (
        <div className="flex max-w-md flex-col gap-4">
            {Array.from({ length: 4 }).map((_, i) => (
                <div key={i} className="flex items-center gap-3">
                    <Skeleton className="h-8 w-8 rounded-md" />
                    <div className="space-y-1">
                        <Skeleton className="h-4 w-36" />
                        <Skeleton className="h-3 w-20" />
                    </div>
                </div>
            ))}
        </div>
    ),
}

function ToastDemo() {
    useEffect(() => {
        toast("Campaign duplicated", { duration: Infinity })
        toast.success("Integration saved", { duration: Infinity })
        toast.error("Failed to switch organization.", { duration: Infinity })
        return () => {
            toast.dismiss()
        }
    }, [])

    return (
        <div className="flex gap-3">
            <Button variant="outline" onClick={() => toast("Campaign duplicated")}>
                Show toast
            </Button>
            <Button variant="outline" onClick={() => toast.error("Something went wrong")}>
                Show error
            </Button>
            <Toaster expand />
        </div>
    )
}

export const Toasts: Story = {
    parameters: { layout: "fullscreen" },
    render: () => (
        <div className="min-h-[360px] p-4">
            <ToastDemo />
        </div>
    ),
}
