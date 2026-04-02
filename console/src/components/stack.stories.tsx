import type { Meta, StoryObj } from "@storybook/react"
import Stack from "./stack"
import { Button } from "@/components/ui/button"

const meta: Meta<typeof Stack> = {
    title: "Components/Stack",
    component: Stack,
    tags: ["autodocs"],
}

export default meta
type Story = StoryObj<typeof Stack>

export const Horizontal: Story = {
    render: () => (
        <Stack>
            <Button variant="default">Primary</Button>
            <Button variant="outline">Secondary</Button>
            <Button variant="ghost">Ghost</Button>
        </Stack>
    ),
}

export const Vertical: Story = {
    render: () => (
        <Stack vertical>
            <Button variant="default">Step 1</Button>
            <Button variant="outline">Step 2</Button>
            <Button variant="ghost">Step 3</Button>
        </Stack>
    ),
}

export const HorizontalWrapping: Story = {
    render: () => (
        <div className="w-60">
            <Stack>
                <Button size="sm">Tag One</Button>
                <Button size="sm">Tag Two</Button>
                <Button size="sm">Tag Three</Button>
                <Button size="sm">Tag Four</Button>
                <Button size="sm">Tag Five</Button>
            </Stack>
        </div>
    ),
}
