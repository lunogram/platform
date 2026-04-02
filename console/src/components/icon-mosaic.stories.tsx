import type { Meta, StoryObj } from "@storybook/react"
import { StaggeredMosaic } from "./icon-mosaic"

const meta: Meta<typeof StaggeredMosaic> = {
    title: "Components/IconMosaic",
    component: StaggeredMosaic,
    tags: ["autodocs"],
}

export default meta
type Story = StoryObj<typeof StaggeredMosaic>

export const WithProvider: Story = {
    render: () => (
        <div className="h-64 w-full overflow-hidden rounded-lg border bg-background">
            <StaggeredMosaic
                provider={{
                    id: "google",
                    name: "Google",
                    icon: "https://www.google.com/favicon.ico",
                    color: "#4285F4",
                }}
            />
        </div>
    ),
}

export const CustomColor: Story = {
    render: () => (
        <div className="h-64 w-full overflow-hidden rounded-lg border bg-background">
            <StaggeredMosaic
                provider={{
                    id: "stripe",
                    name: "Stripe",
                    color: "#635BFF",
                }}
            />
        </div>
    ),
}

export const NoProvider: Story = {
    render: () => (
        <div className="h-64 w-full overflow-hidden rounded-lg border bg-background">
            <StaggeredMosaic />
        </div>
    ),
}

export const SmallGrid: Story = {
    render: () => (
        <div className="h-48 w-full overflow-hidden rounded-lg border bg-background">
            <StaggeredMosaic
                provider={{ id: "github", name: "GitHub", color: "#24292e" }}
                cols={5}
                rows={3}
            />
        </div>
    ),
}
