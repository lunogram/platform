import type { Meta, StoryObj } from "@storybook/react"
import { useState } from "react"
import { Home, Settings, Users, Bell } from "lucide-react"
import { NavTabs } from "./nav-tabs"
import type { NavTab } from "./nav-tabs"

const meta: Meta<typeof NavTabs> = {
    title: "UI/NavTabs",
    component: NavTabs,
    tags: ["autodocs"],
}

export default meta
type Story = StoryObj<typeof NavTabs>

const tabs: NavTab[] = [
    { key: "overview", label: "Overview", icon: Home },
    { key: "members", label: "Members", icon: Users, badge: 3 },
    { key: "settings", label: "Settings", icon: Settings },
    { key: "notifications", label: "Notifications", icon: Bell, badge: 12 },
]

export const StateBased: Story = {
    render: () => {
        const [value, setValue] = useState("overview")
        return (
            <div className="border-b">
                <NavTabs tabs={tabs} value={value} onChange={setValue} />
            </div>
        )
    },
}

export const WithBadges: Story = {
    render: () => {
        const [value, setValue] = useState("members")
        return (
            <div className="border-b">
                <NavTabs tabs={tabs} value={value} onChange={setValue} />
            </div>
        )
    },
}

export const TwoTabs: Story = {
    render: () => {
        const [value, setValue] = useState("general")
        const simpleTabs: NavTab[] = [
            { key: "general", label: "General", icon: Settings },
            { key: "security", label: "Security", icon: Bell },
        ]
        return (
            <div className="border-b">
                <NavTabs tabs={simpleTabs} value={value} onChange={setValue} />
            </div>
        )
    },
}
