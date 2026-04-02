import { useState } from "react"
import type { Meta, StoryObj } from "@storybook/react"
import {
    Select,
    SelectTrigger,
    SelectValue,
    SelectContent,
    SelectItem,
    SelectLabel,
    SelectGroup,
    SelectSeparator,
} from "./select"

const meta: Meta<typeof Select> = {
    component: Select,
    tags: ["autodocs"],
}
export default meta

type Story = StoryObj<typeof Select>

export const Default: Story = {
    render: () => {
        const [value, setValue] = useState("")
        return (
            <Select value={value} onValueChange={setValue}>
                <SelectTrigger style={{ width: "240px" }}>
                    <SelectValue placeholder="Select a fruit" />
                </SelectTrigger>
                <SelectContent>
                    <SelectItem value="apple">Apple</SelectItem>
                    <SelectItem value="banana">Banana</SelectItem>
                    <SelectItem value="orange">Orange</SelectItem>
                    <SelectItem value="grape">Grape</SelectItem>
                </SelectContent>
            </Select>
        )
    },
}

export const WithGroups: Story = {
    render: () => {
        const [value, setValue] = useState("")
        return (
            <Select value={value} onValueChange={setValue}>
                <SelectTrigger style={{ width: "240px" }}>
                    <SelectValue placeholder="Select a timezone" />
                </SelectTrigger>
                <SelectContent>
                    <SelectGroup>
                        <SelectLabel>North America</SelectLabel>
                        <SelectItem value="est">Eastern Time (EST)</SelectItem>
                        <SelectItem value="cst">Central Time (CST)</SelectItem>
                        <SelectItem value="pst">Pacific Time (PST)</SelectItem>
                    </SelectGroup>
                    <SelectSeparator />
                    <SelectGroup>
                        <SelectLabel>Europe</SelectLabel>
                        <SelectItem value="gmt">Greenwich Mean Time (GMT)</SelectItem>
                        <SelectItem value="cet">Central European Time (CET)</SelectItem>
                    </SelectGroup>
                </SelectContent>
            </Select>
        )
    },
}

export const WithPreselectedValue: Story = {
    render: () => {
        const [value, setValue] = useState("banana")
        return (
            <Select value={value} onValueChange={setValue}>
                <SelectTrigger style={{ width: "240px" }}>
                    <SelectValue />
                </SelectTrigger>
                <SelectContent>
                    <SelectItem value="apple">Apple</SelectItem>
                    <SelectItem value="banana">Banana</SelectItem>
                    <SelectItem value="orange">Orange</SelectItem>
                </SelectContent>
            </Select>
        )
    },
}
