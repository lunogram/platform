import type { Meta, StoryObj } from "@storybook/react"
import { Skeleton } from "./skeleton"

const meta: Meta<typeof Skeleton> = {
    component: Skeleton,
    tags: ["autodocs"],
}
export default meta

type Story = StoryObj<typeof Skeleton>

export const Default: Story = {
    args: {
        style: { width: "200px", height: "20px" },
    },
}

export const CardSkeleton: Story = {
    render: () => (
        <div style={{ display: "flex", flexDirection: "column", gap: "12px", width: "300px" }}>
            <Skeleton style={{ height: "160px", width: "100%" }} />
            <Skeleton style={{ height: "20px", width: "80%" }} />
            <Skeleton style={{ height: "16px", width: "60%" }} />
            <Skeleton style={{ height: "16px", width: "70%" }} />
        </div>
    ),
}

export const ProfileSkeleton: Story = {
    render: () => (
        <div style={{ display: "flex", alignItems: "center", gap: "12px" }}>
            <Skeleton style={{ width: "48px", height: "48px", borderRadius: "50%" }} />
            <div style={{ display: "flex", flexDirection: "column", gap: "6px" }}>
                <Skeleton style={{ height: "16px", width: "120px" }} />
                <Skeleton style={{ height: "14px", width: "80px" }} />
            </div>
        </div>
    ),
}
