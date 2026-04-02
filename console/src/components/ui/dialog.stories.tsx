import type { Meta, StoryObj } from "@storybook/react"
import {
    Dialog,
    DialogTrigger,
    DialogContent,
    DialogHeader,
    DialogFooter,
    DialogTitle,
    DialogDescription,
    DialogClose,
} from "./dialog"
import { Button } from "./button"

const meta: Meta<typeof Dialog> = {
    component: Dialog,
    tags: ["autodocs"],
}
export default meta

type Story = StoryObj<typeof Dialog>

export const Default: Story = {
    render: () => (
        <Dialog>
            <DialogTrigger asChild>
                <Button>Open Dialog</Button>
            </DialogTrigger>
            <DialogContent>
                <DialogHeader>
                    <DialogTitle>Confirm Action</DialogTitle>
                    <DialogDescription>
                        Are you sure you want to proceed? This action cannot be undone.
                    </DialogDescription>
                </DialogHeader>
                <DialogFooter>
                    <DialogClose asChild>
                        <Button variant="outline">Cancel</Button>
                    </DialogClose>
                    <Button>Confirm</Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    ),
}

export const WithoutCloseButton: Story = {
    render: () => (
        <Dialog>
            <DialogTrigger asChild>
                <Button variant="outline">Open (no close icon)</Button>
            </DialogTrigger>
            <DialogContent showClose={false}>
                <DialogHeader>
                    <DialogTitle>No Close Icon</DialogTitle>
                    <DialogDescription>
                        This dialog hides the default close icon. Use the button below to dismiss.
                    </DialogDescription>
                </DialogHeader>
                <DialogFooter>
                    <DialogClose asChild>
                        <Button>Dismiss</Button>
                    </DialogClose>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    ),
}
