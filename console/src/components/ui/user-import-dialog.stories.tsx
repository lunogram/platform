import type { Meta, StoryObj } from "@storybook/react"
import { useState } from "react"
import { UserImportDialog } from "./user-import-dialog"
import { Button } from "./button"

const meta: Meta<typeof UserImportDialog> = {
    title: "UI/UserImportDialog",
    component: UserImportDialog,
    tags: ["autodocs"],
}

export default meta
type Story = StoryObj<typeof UserImportDialog>

export const Default: Story = {
    render: () => {
        const [open, setOpen] = useState(false)
        return (
            <div className="p-4">
                <Button onClick={() => setOpen(true)}>Import Users</Button>
                <UserImportDialog
                    open={open}
                    onOpenChange={setOpen}
                    onImport={async (file) => {
                        await new Promise((r) => setTimeout(r, 1500))
                        console.log("Imported file:", file.name)
                    }}
                />
            </div>
        )
    },
}

export const OpenByDefault: Story = {
    render: () => {
        const [open, setOpen] = useState(true)
        return (
            <UserImportDialog
                open={open}
                onOpenChange={setOpen}
                onImport={async (file) => {
                    await new Promise((r) => setTimeout(r, 1500))
                    console.log("Imported file:", file.name)
                }}
            />
        )
    },
}
