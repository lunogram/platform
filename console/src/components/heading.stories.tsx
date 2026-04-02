import type { Meta, StoryObj } from "@storybook/react"
import Heading from "./heading"
import { Button } from "@/components/ui/button"

const meta: Meta<typeof Heading> = {
    title: "Components/Heading",
    component: Heading,
    tags: ["autodocs"],
}

export default meta
type Story = StoryObj<typeof Heading>

export const H2: Story = {
    render: () => (
        <Heading
            title="Users"
            size="h2"
            actions={<Button>Add User</Button>}
        >
            Manage all users in your organisation.
        </Heading>
    ),
}

export const H3: Story = {
    render: () => (
        <Heading
            title="Account Settings"
            size="h3"
            actions={<Button variant="outline" size="sm">Edit</Button>}
        />
    ),
}

export const H4: Story = {
    render: () => <Heading title="API Keys" size="h4" />,
}

export const H5: Story = {
    render: () => <Heading title="Environment Variables" size="h5" />,
}

export const WithActionsAndChildren: Story = {
    render: () => (
        <Heading
            title="Deployments"
            size="h2"
            actions={
                <>
                    <Button variant="outline">Refresh</Button>
                    <Button>New Deployment</Button>
                </>
            }
        >
            View and manage your recent deployments.
        </Heading>
    ),
}
