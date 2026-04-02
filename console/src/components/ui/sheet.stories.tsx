import type { Meta, StoryObj } from "@storybook/react"
import {
    Sheet,
    SheetTrigger,
    SheetContent,
    SheetHeader,
    SheetTitle,
    SheetDescription,
    SheetFooter,
    SheetClose,
} from "./sheet"
import { Button } from "./button"

const meta: Meta<typeof Sheet> = {
    component: Sheet,
    tags: ["autodocs"],
}
export default meta

type Story = StoryObj<typeof Sheet>

export const Right: Story = {
    render: () => (
        <Sheet>
            <SheetTrigger asChild>
                <Button variant="outline">Open Right Sheet</Button>
            </SheetTrigger>
            <SheetContent side="right">
                <SheetHeader>
                    <SheetTitle>Edit Profile</SheetTitle>
                    <SheetDescription>
                        Make changes to your profile here. Click save when done.
                    </SheetDescription>
                </SheetHeader>
                <SheetFooter>
                    <SheetClose asChild>
                        <Button variant="outline">Cancel</Button>
                    </SheetClose>
                    <Button>Save changes</Button>
                </SheetFooter>
            </SheetContent>
        </Sheet>
    ),
}

export const Left: Story = {
    render: () => (
        <Sheet>
            <SheetTrigger asChild>
                <Button variant="outline">Open Left Sheet</Button>
            </SheetTrigger>
            <SheetContent side="left">
                <SheetHeader>
                    <SheetTitle>Navigation</SheetTitle>
                    <SheetDescription>Main application navigation.</SheetDescription>
                </SheetHeader>
                <div style={{ display: "flex", flexDirection: "column", gap: "8px", paddingTop: "16px" }}>
                    <Button variant="ghost" style={{ justifyContent: "flex-start" }}>Dashboard</Button>
                    <Button variant="ghost" style={{ justifyContent: "flex-start" }}>Projects</Button>
                    <Button variant="ghost" style={{ justifyContent: "flex-start" }}>Settings</Button>
                </div>
            </SheetContent>
        </Sheet>
    ),
}

export const Bottom: Story = {
    render: () => (
        <Sheet>
            <SheetTrigger asChild>
                <Button variant="outline">Open Bottom Sheet</Button>
            </SheetTrigger>
            <SheetContent side="bottom">
                <SheetHeader>
                    <SheetTitle>Confirm Deletion</SheetTitle>
                    <SheetDescription>
                        This action cannot be undone. The item will be permanently deleted.
                    </SheetDescription>
                </SheetHeader>
                <SheetFooter style={{ marginTop: "16px" }}>
                    <SheetClose asChild>
                        <Button variant="outline">Cancel</Button>
                    </SheetClose>
                    <Button variant="destructive">Delete</Button>
                </SheetFooter>
            </SheetContent>
        </Sheet>
    ),
}
