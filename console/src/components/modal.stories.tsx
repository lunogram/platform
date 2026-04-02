import type { Meta, StoryObj } from "@storybook/react"
import { useState } from "react"
import Modal from "./modal"
import { Button } from "@/components/ui/button"

const meta: Meta<typeof Modal> = {
    title: "Components/Modal",
    component: Modal,
    tags: ["autodocs"],
}

export default meta
type Story = StoryObj<typeof Modal>

export const Small: Story = {
    render: () => {
        const [open, setOpen] = useState(false)
        return (
            <div className="p-4">
                <Button onClick={() => setOpen(true)}>Open Small Modal</Button>
                <Modal
                    open={open}
                    onClose={setOpen}
                    title="Delete Item"
                    description="Are you sure you want to delete this item? This action cannot be undone."
                    size="small"
                    actions={
                        <div className="flex justify-end gap-2">
                            <Button variant="outline" onClick={() => setOpen(false)}>
                                Cancel
                            </Button>
                            <Button variant="destructive" onClick={() => setOpen(false)}>
                                Delete
                            </Button>
                        </div>
                    }
                />
            </div>
        )
    },
}

export const Regular: Story = {
    render: () => {
        const [open, setOpen] = useState(false)
        return (
            <div className="p-4">
                <Button onClick={() => setOpen(true)}>Open Regular Modal</Button>
                <Modal
                    open={open}
                    onClose={setOpen}
                    title="Edit Profile"
                    description="Update your profile information below."
                    size="regular"
                    actions={
                        <div className="flex justify-end gap-2">
                            <Button variant="outline" onClick={() => setOpen(false)}>
                                Cancel
                            </Button>
                            <Button onClick={() => setOpen(false)}>Save Changes</Button>
                        </div>
                    }
                >
                    <div className="space-y-4">
                        <div className="grid gap-1.5">
                            <label className="text-sm font-medium">Name</label>
                            <input
                                className="rounded-md border px-3 py-2 text-sm"
                                defaultValue="Alice Smith"
                            />
                        </div>
                        <div className="grid gap-1.5">
                            <label className="text-sm font-medium">Email</label>
                            <input
                                className="rounded-md border px-3 py-2 text-sm"
                                defaultValue="alice@example.com"
                                type="email"
                            />
                        </div>
                    </div>
                </Modal>
            </div>
        )
    },
}

export const Large: Story = {
    render: () => {
        const [open, setOpen] = useState(false)
        return (
            <div className="p-4">
                <Button onClick={() => setOpen(true)}>Open Large Modal</Button>
                <Modal
                    open={open}
                    onClose={setOpen}
                    title="Import Data"
                    description="Upload a file to import records into the system."
                    size="large"
                    actions={
                        <div className="flex justify-end gap-2">
                            <Button variant="outline" onClick={() => setOpen(false)}>
                                Cancel
                            </Button>
                            <Button onClick={() => setOpen(false)}>Import</Button>
                        </div>
                    }
                >
                    <div className="rounded-lg border border-dashed p-12 text-center text-muted-foreground">
                        Drop a file here or click to browse
                    </div>
                </Modal>
            </div>
        )
    },
}
