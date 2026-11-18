import type { Meta, StoryObj } from "@storybook/react-vite"

import { Button } from "@/components/ui/button"
import {
    Card,
    CardContent,
    CardDescription,
    CardFooter,
    CardHeader,
    CardTitle,
} from "@/components/ui/card"

const meta: Meta<typeof Card> = {
    title: "UI/Card",
    component: Card,
}

export default meta
type Story = StoryObj<typeof Card>

export const Default: Story = {
    render: () => (
        <Card className="max-w-md">
            <CardHeader>
                <CardTitle>Project details</CardTitle>
                <CardDescription>Name, locale, and timezone used for scheduling.</CardDescription>
            </CardHeader>
            <CardContent className="text-sm">
                Messages scheduled for 9:00 are delivered in each user&apos;s local time.
            </CardContent>
            <CardFooter className="justify-end gap-2">
                <Button variant="outline">Cancel</Button>
                <Button>Save</Button>
            </CardFooter>
        </Card>
    ),
}

export const Stats: Story = {
    render: () => (
        <div className="grid max-w-3xl grid-cols-3 gap-4">
            {[
                ["Sent", "48,210"],
                ["Opened", "20,248"],
                ["Clicked", "3,857"],
            ].map(([label, value]) => (
                <Card key={label}>
                    <CardHeader className="pb-2">
                        <CardDescription>{label}</CardDescription>
                        <CardTitle className="text-2xl">{value}</CardTitle>
                    </CardHeader>
                </Card>
            ))}
        </div>
    ),
}
