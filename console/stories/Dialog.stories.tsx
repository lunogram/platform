import type { Meta, StoryObj } from "@storybook/react-vite"
import { useState } from "react"

import {
    AlertDialog,
    AlertDialogAction,
    AlertDialogCancel,
    AlertDialogContent,
    AlertDialogDescription,
    AlertDialogFooter,
    AlertDialogHeader,
    AlertDialogTitle,
    AlertDialogTrigger,
} from "@/components/ui/alert-dialog"
import { Button } from "@/components/ui/button"
import {
    Dialog,
    DialogClose,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
    DialogTrigger,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
    Sheet,
    SheetClose,
    SheetContent,
    SheetDescription,
    SheetFooter,
    SheetHeader,
    SheetTitle,
    SheetTrigger,
} from "@/components/ui/sheet"
import { UserImportDialog, UserImportForm } from "@/components/ui/user-import-dialog"

const meta: Meta = {
    title: "UI/Dialog",
    parameters: { layout: "fullscreen" },
}

export default meta
type Story = StoryObj

export const DialogStory: Story = {
    name: "Dialog",
    render: () => (
        <Dialog defaultOpen>
            <DialogTrigger asChild>
                <Button variant="outline" className="m-4">
                    Rename list
                </Button>
            </DialogTrigger>
            <DialogContent>
                <DialogHeader>
                    <DialogTitle>Rename list</DialogTitle>
                    <DialogDescription>
                        The new name is shown everywhere the list is used.
                    </DialogDescription>
                </DialogHeader>
                <div className="grid gap-2">
                    <Label htmlFor="list-name">Name</Label>
                    <Input id="list-name" defaultValue="Beta testers" />
                </div>
                <DialogFooter>
                    <DialogClose asChild>
                        <Button variant="outline">Cancel</Button>
                    </DialogClose>
                    <Button>Save</Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    ),
}

export const Confirm: Story = {
    render: () => (
        <AlertDialog defaultOpen>
            <AlertDialogTrigger asChild>
                <Button variant="destructive" className="m-4">
                    Delete list
                </Button>
            </AlertDialogTrigger>
            <AlertDialogContent>
                <AlertDialogHeader>
                    <AlertDialogTitle>Delete this list?</AlertDialogTitle>
                    <AlertDialogDescription>
                        Campaigns and journeys that target it stop sending. This cannot be undone.
                    </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                    <AlertDialogCancel>Cancel</AlertDialogCancel>
                    <AlertDialogAction className="bg-destructive hover:bg-destructive/90">
                        Delete
                    </AlertDialogAction>
                </AlertDialogFooter>
            </AlertDialogContent>
        </AlertDialog>
    ),
}

export const SheetStory: Story = {
    name: "Sheet",
    render: () => (
        <Sheet defaultOpen>
            <SheetTrigger asChild>
                <Button variant="outline" className="m-4">
                    Open details
                </Button>
            </SheetTrigger>
            <SheetContent>
                <SheetHeader>
                    <SheetTitle>Event details</SheetTitle>
                    <SheetDescription>order_completed · Nov 14, 2026, 10:24 AM</SheetDescription>
                </SheetHeader>
                <div className="grid gap-2 py-6 text-sm">
                    <div className="flex justify-between">
                        <span className="text-muted-foreground">Total</span>
                        <span>$128.40</span>
                    </div>
                    <div className="flex justify-between">
                        <span className="text-muted-foreground">Currency</span>
                        <span>USD</span>
                    </div>
                </div>
                <SheetFooter>
                    <SheetClose asChild>
                        <Button variant="outline">Close</Button>
                    </SheetClose>
                </SheetFooter>
            </SheetContent>
        </Sheet>
    ),
}

function UserImportDemo() {
    const [open, setOpen] = useState(true)
    return (
        <>
            <Button variant="outline" className="m-4" onClick={() => setOpen(true)}>
                Import users
            </Button>
            <UserImportDialog
                open={open}
                onOpenChange={setOpen}
                onImport={() => Promise.resolve()}
            />
        </>
    )
}

export const UserImport: Story = {
    name: "User import",
    render: () => <UserImportDemo />,
}

export const UserImportFileSelected: Story = {
    name: "User import / file selected",
    parameters: { layout: "padded" },
    render: () => (
        <div className="max-w-lg">
            <UserImportForm
                file={new File(["external_id,email\n"], "users.csv", { type: "text/csv" })}
                onFileChange={() => {}}
            />
        </div>
    ),
}
