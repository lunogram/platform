import type { Meta, StoryObj } from "@storybook/react"
import { Tabs, TabsList, TabsTrigger, TabsContent } from "./tabs"

const meta: Meta<typeof Tabs> = {
    component: Tabs,
    tags: ["autodocs"],
}
export default meta

type Story = StoryObj<typeof Tabs>

export const Default: Story = {
    render: () => (
        <Tabs defaultValue="account" style={{ width: "400px" }}>
            <TabsList>
                <TabsTrigger value="account">Account</TabsTrigger>
                <TabsTrigger value="password">Password</TabsTrigger>
            </TabsList>
            <TabsContent value="account">
                <p style={{ fontSize: "14px", padding: "8px 0" }}>
                    Make changes to your account here. Click save when you are done.
                </p>
            </TabsContent>
            <TabsContent value="password">
                <p style={{ fontSize: "14px", padding: "8px 0" }}>
                    Change your password here. After saving, you will be logged out.
                </p>
            </TabsContent>
        </Tabs>
    ),
}

export const ThreeTabs: Story = {
    render: () => (
        <Tabs defaultValue="overview" style={{ width: "500px" }}>
            <TabsList>
                <TabsTrigger value="overview">Overview</TabsTrigger>
                <TabsTrigger value="analytics">Analytics</TabsTrigger>
                <TabsTrigger value="settings">Settings</TabsTrigger>
            </TabsList>
            <TabsContent value="overview">
                <p style={{ fontSize: "14px", padding: "8px 0" }}>Overview content goes here.</p>
            </TabsContent>
            <TabsContent value="analytics">
                <p style={{ fontSize: "14px", padding: "8px 0" }}>Analytics content goes here.</p>
            </TabsContent>
            <TabsContent value="settings">
                <p style={{ fontSize: "14px", padding: "8px 0" }}>Settings content goes here.</p>
            </TabsContent>
        </Tabs>
    ),
}
