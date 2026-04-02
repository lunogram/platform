import type { Meta, StoryObj } from "@storybook/react"
import {
    Card,
    CardHeader,
    CardTitle,
    CardDescription,
    CardContent,
    CardFooter,
} from "./card"
import { Button } from "./button"

const meta: Meta<typeof Card> = {
    component: Card,
    tags: ["autodocs"],
}
export default meta

type Story = StoryObj<typeof Card>

export const Default: Story = {
    render: () => (
        <Card style={{ width: "360px" }}>
            <CardHeader>
                <CardTitle>Card Title</CardTitle>
                <CardDescription>Card description goes here.</CardDescription>
            </CardHeader>
            <CardContent>
                <p>This is the main content of the card.</p>
            </CardContent>
            <CardFooter style={{ display: "flex", justifyContent: "flex-end", gap: "8px" }}>
                <Button variant="outline">Cancel</Button>
                <Button>Save</Button>
            </CardFooter>
        </Card>
    ),
}

export const SimpleContent: Story = {
    render: () => (
        <Card style={{ width: "360px" }}>
            <CardHeader>
                <CardTitle>Notifications</CardTitle>
            </CardHeader>
            <CardContent>
                <p>You have 3 unread messages.</p>
            </CardContent>
        </Card>
    ),
}
