import type { Meta, StoryObj } from "@storybook/react"
import { Avatar, AvatarImage, AvatarFallback } from "./avatar"

const meta: Meta<typeof Avatar> = {
    component: Avatar,
    tags: ["autodocs"],
}
export default meta

type Story = StoryObj<typeof Avatar>

export const WithImage: Story = {
    render: () => (
        <Avatar>
            <AvatarImage src="https://github.com/shadcn.png" alt="@shadcn" />
            <AvatarFallback>CN</AvatarFallback>
        </Avatar>
    ),
}

export const FallbackOnly: Story = {
    render: () => (
        <Avatar>
            <AvatarImage src="" alt="broken" />
            <AvatarFallback>AB</AvatarFallback>
        </Avatar>
    ),
}

export const Initials: Story = {
    render: () => (
        <div style={{ display: "flex", gap: "8px" }}>
            <Avatar>
                <AvatarFallback>JD</AvatarFallback>
            </Avatar>
            <Avatar>
                <AvatarFallback>MK</AvatarFallback>
            </Avatar>
            <Avatar>
                <AvatarFallback>SB</AvatarFallback>
            </Avatar>
        </div>
    ),
}
