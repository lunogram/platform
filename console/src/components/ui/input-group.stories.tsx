import type { Meta, StoryObj } from "@storybook/react"
import { Search } from "lucide-react"
import {
    InputGroup,
    InputGroupAddon,
    InputGroupInput,
    InputGroupText,
    InputGroupButton,
    InputGroupTextarea,
} from "./input-group"

const meta: Meta<typeof InputGroup> = {
    title: "UI/InputGroup",
    component: InputGroup,
    tags: ["autodocs"],
}

export default meta
type Story = StoryObj<typeof InputGroup>

export const CurrencyAmount: Story = {
    render: () => (
        <InputGroup>
            <InputGroupAddon align="inline-start">
                <InputGroupText>$</InputGroupText>
            </InputGroupAddon>
            <InputGroupInput placeholder="Amount" type="number" />
            <InputGroupAddon align="inline-end">
                <InputGroupText>.00</InputGroupText>
            </InputGroupAddon>
        </InputGroup>
    ),
}

export const SearchWithButton: Story = {
    render: () => (
        <InputGroup>
            <InputGroupAddon align="inline-start">
                <InputGroupText>
                    <Search className="h-4 w-4" />
                </InputGroupText>
            </InputGroupAddon>
            <InputGroupInput placeholder="Search..." />
            <InputGroupAddon align="inline-end">
                <InputGroupButton size="xs">Go</InputGroupButton>
            </InputGroupAddon>
        </InputGroup>
    ),
}

export const UrlInput: Story = {
    render: () => (
        <InputGroup>
            <InputGroupAddon align="inline-start">
                <InputGroupText>https://</InputGroupText>
            </InputGroupAddon>
            <InputGroupInput placeholder="your-domain.com" />
        </InputGroup>
    ),
}

export const BlockStartLabel: Story = {
    render: () => (
        <InputGroup>
            <InputGroupAddon align="block-start">
                <InputGroupText>Message</InputGroupText>
            </InputGroupAddon>
            <InputGroupTextarea placeholder="Type your message here..." rows={3} />
        </InputGroup>
    ),
}

export const BlockEndLabel: Story = {
    render: () => (
        <InputGroup>
            <InputGroupInput placeholder="Enter value" />
            <InputGroupAddon align="block-end">
                <InputGroupText>characters remaining: 100</InputGroupText>
            </InputGroupAddon>
        </InputGroup>
    ),
}
