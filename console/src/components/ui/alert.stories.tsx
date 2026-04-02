import type { Meta, StoryObj } from "@storybook/react"
import { Alert, AlertTitle, AlertDescription } from "./alert"

const meta: Meta<typeof Alert> = {
    component: Alert,
    tags: ["autodocs"],
}
export default meta

type Story = StoryObj<typeof Alert>

export const Default: Story = {
    render: () => (
        <Alert>
            <AlertTitle>Heads up!</AlertTitle>
            <AlertDescription>
                You can add components to your app using the cli.
            </AlertDescription>
        </Alert>
    ),
}

export const Destructive: Story = {
    render: () => (
        <Alert variant="destructive">
            <AlertTitle>Error</AlertTitle>
            <AlertDescription>
                Your session has expired. Please log in again.
            </AlertDescription>
        </Alert>
    ),
}

export const TitleOnly: Story = {
    render: () => (
        <Alert>
            <AlertTitle>Note</AlertTitle>
        </Alert>
    ),
}
