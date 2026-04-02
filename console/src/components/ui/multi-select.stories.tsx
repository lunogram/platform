import type { Meta, StoryObj } from "@storybook/react"
import { useState } from "react"
import { MultiSelect } from "./multi-select"

const meta: Meta<typeof MultiSelect> = {
    title: "UI/MultiSelect",
    component: MultiSelect,
    tags: ["autodocs"],
}

export default meta
type Story = StoryObj<typeof MultiSelect>

const options = [
    { value: "react", label: "React" },
    { value: "vue", label: "Vue" },
    { value: "angular", label: "Angular" },
    { value: "svelte", label: "Svelte" },
    { value: "solid", label: "Solid" },
    { value: "qwik", label: "Qwik" },
]

export const Default: Story = {
    render: () => {
        const [value, setValue] = useState<string[]>([])
        return (
            <div className="w-80">
                <MultiSelect
                    options={options}
                    value={value}
                    onChange={setValue}
                    placeholder="Select frameworks..."
                />
            </div>
        )
    },
}

export const WithPreselected: Story = {
    render: () => {
        const [value, setValue] = useState<string[]>(["react", "vue"])
        return (
            <div className="w-80">
                <MultiSelect
                    options={options}
                    value={value}
                    onChange={setValue}
                    placeholder="Select frameworks..."
                />
            </div>
        )
    },
}

export const MaxDisplayTwo: Story = {
    render: () => {
        const [value, setValue] = useState<string[]>(["react", "vue", "angular", "svelte"])
        return (
            <div className="w-80">
                <MultiSelect
                    options={options}
                    value={value}
                    onChange={setValue}
                    placeholder="Select frameworks..."
                    maxDisplay={2}
                />
            </div>
        )
    },
}

export const Disabled: Story = {
    render: () => (
        <div className="w-80">
            <MultiSelect
                options={options}
                value={["react"]}
                onChange={() => {}}
                placeholder="Select frameworks..."
                disabled
            />
        </div>
    ),
}
