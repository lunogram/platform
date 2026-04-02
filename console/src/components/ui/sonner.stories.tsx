import type { Meta, StoryObj } from "@storybook/react"
import { toast } from "sonner"
import { Toaster } from "./sonner"
import { Button } from "./button"

const meta: Meta<typeof Toaster> = {
    component: Toaster,
    tags: ["autodocs"],
    decorators: [
        (Story) => (
            <>
                <Story />
                <div style={{ display: "flex", gap: "8px", flexWrap: "wrap", padding: "16px" }}>
                    <Button onClick={() => toast("Event has been created")}>Default Toast</Button>
                    <Button variant="secondary" onClick={() => toast.success("Profile saved successfully!")}>
                        Success
                    </Button>
                    <Button variant="destructive" onClick={() => toast.error("Something went wrong.")}>
                        Error
                    </Button>
                    <Button variant="outline" onClick={() => toast.warning("Your session is about to expire.")}>
                        Warning
                    </Button>
                    <Button variant="outline" onClick={() => toast.info("A new update is available.")}>
                        Info
                    </Button>
                </div>
            </>
        ),
    ],
}
export default meta

type Story = StoryObj<typeof Toaster>

export const Default: Story = {
    render: () => <Toaster />,
}

export const RichColors: Story = {
    render: () => <Toaster richColors />,
}

export const TopCenter: Story = {
    render: () => <Toaster position="top-center" />,
}
