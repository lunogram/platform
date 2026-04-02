import type { Meta, StoryObj } from "@storybook/react"
import { Tooltip, TooltipTrigger, TooltipContent, TooltipProvider } from "./tooltip"
import { Button } from "./button"

const meta: Meta<typeof Tooltip> = {
    component: Tooltip,
    tags: ["autodocs"],
    decorators: [
        (Story) => (
            <TooltipProvider>
                <Story />
            </TooltipProvider>
        ),
    ],
}
export default meta

type Story = StoryObj<typeof Tooltip>

export const Default: Story = {
    render: () => (
        <Tooltip>
            <TooltipTrigger asChild>
                <Button variant="outline">Hover me</Button>
            </TooltipTrigger>
            <TooltipContent>
                <p>This is a tooltip</p>
            </TooltipContent>
        </Tooltip>
    ),
}

export const WithLongText: Story = {
    render: () => (
        <Tooltip>
            <TooltipTrigger asChild>
                <Button variant="ghost">More info</Button>
            </TooltipTrigger>
            <TooltipContent>
                <p>This tooltip contains a longer description to explain something in detail.</p>
            </TooltipContent>
        </Tooltip>
    ),
}

export const AboveElement: Story = {
    render: () => (
        <Tooltip>
            <TooltipTrigger asChild>
                <Button>Top tooltip</Button>
            </TooltipTrigger>
            <TooltipContent side="top">
                <p>Appears above</p>
            </TooltipContent>
        </Tooltip>
    ),
}
