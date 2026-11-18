import type { Meta, StoryObj } from "@storybook/react-vite"
import { ArrowRight, Plus, Trash2 } from "lucide-react"
import { Link } from "react-router"

import { Button } from "@/components/ui/button"

const meta: Meta<typeof Button> = {
    title: "UI/Button",
    component: Button,
}

export default meta
type Story = StoryObj<typeof Button>

export const Variants: Story = {
    render: () => (
        <div className="flex flex-wrap items-center gap-3">
            <Button>Default</Button>
            <Button variant="secondary">Secondary</Button>
            <Button variant="outline">Outline</Button>
            <Button variant="ghost">Ghost</Button>
            <Button variant="link">Link</Button>
            <Button variant="destructive">Destructive</Button>
        </div>
    ),
}

export const Sizes: Story = {
    render: () => (
        <div className="flex flex-wrap items-center gap-3">
            <Button size="lg">Large</Button>
            <Button>Default</Button>
            <Button size="sm">Small</Button>
            <Button size="icon" variant="outline" aria-label="Add">
                <Plus />
            </Button>
            <Button size="min" variant="ghost" aria-label="Delete">
                <Trash2 />
            </Button>
        </div>
    ),
}

export const States: Story = {
    render: () => (
        <div className="flex flex-wrap items-center gap-3">
            <Button disabled>Disabled</Button>
            <Button variant="outline" disabled>
                Disabled outline
            </Button>
            <Button isLoading disabled>
                Saving...
            </Button>
            <Button variant="outline" isLoading disabled>
                Loading
            </Button>
        </div>
    ),
}

export const WithIcons: Story = {
    render: () => (
        <div className="flex flex-wrap items-center gap-3">
            <Button>
                <Plus /> Create Campaign
            </Button>
            <Button variant="outline">
                Continue <ArrowRight />
            </Button>
            <Button variant="destructive">
                <Trash2 /> Delete
            </Button>
        </div>
    ),
}

export const AsLink: Story = {
    render: () => (
        <Button asChild variant="outline">
            <Link to="/projects">Go to projects</Link>
        </Button>
    ),
}
