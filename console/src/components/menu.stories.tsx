import type { Meta, StoryObj } from "@storybook/react"
import { Edit, Trash2, Copy, ExternalLink } from "lucide-react"
import Menu, { MenuItem } from "./menu"
import { Button } from "@/components/ui/button"

const meta: Meta<typeof Menu> = {
    title: "Components/Menu",
    component: Menu,
    tags: ["autodocs"],
}

export default meta
type Story = StoryObj<typeof Menu>

export const Default: Story = {
    render: () => (
        <div className="flex items-center justify-center p-8">
            <Menu>
                <MenuItem onClick={() => console.log("Edit")}>
                    <Edit />
                    Edit
                </MenuItem>
                <MenuItem onClick={() => console.log("Duplicate")}>
                    <Copy />
                    Duplicate
                </MenuItem>
                <MenuItem onClick={() => console.log("Open")}>
                    <ExternalLink />
                    Open in new tab
                </MenuItem>
                <MenuItem onClick={() => console.log("Delete")}>
                    <Trash2 />
                    Delete
                </MenuItem>
            </Menu>
        </div>
    ),
}

export const CustomButton: Story = {
    render: () => (
        <div className="flex items-center justify-center p-8">
            <Menu button={<Button variant="outline">Actions</Button>}>
                <MenuItem onClick={() => console.log("Edit")}>
                    <Edit />
                    Edit
                </MenuItem>
                <MenuItem onClick={() => console.log("Delete")}>
                    <Trash2 />
                    Delete
                </MenuItem>
            </Menu>
        </div>
    ),
}

export const GhostVariant: Story = {
    render: () => (
        <div className="flex items-center justify-center p-8">
            <Menu variant="ghost">
                <MenuItem onClick={() => console.log("Copy")}>
                    <Copy />
                    Copy ID
                </MenuItem>
                <MenuItem onClick={() => console.log("Edit")}>
                    <Edit />
                    Edit record
                </MenuItem>
                <MenuItem onClick={() => console.log("Delete")}>
                    <Trash2 />
                    Remove
                </MenuItem>
            </Menu>
        </div>
    ),
}
