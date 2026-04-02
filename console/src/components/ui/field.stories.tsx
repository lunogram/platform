import type { Meta, StoryObj } from "@storybook/react"
import {
    Field,
    FieldLabel,
    FieldDescription,
    FieldError,
    FieldGroup,
    FieldSet,
    FieldContent,
    FieldTitle,
    FieldLegend,
    FieldSeparator,
} from "./field"
import { Input } from "./input"

const meta: Meta<typeof Field> = {
    title: "UI/Field",
    component: Field,
    tags: ["autodocs"],
}

export default meta
type Story = StoryObj<typeof Field>

export const Vertical: Story = {
    render: () => (
        <Field orientation="vertical">
            <FieldLabel>Name</FieldLabel>
            <FieldContent>
                <Input placeholder="John Doe" />
            </FieldContent>
            <FieldDescription>Your full name.</FieldDescription>
        </Field>
    ),
}

export const Horizontal: Story = {
    render: () => (
        <Field orientation="horizontal">
            <FieldLabel>Email</FieldLabel>
            <FieldContent>
                <Input placeholder="you@example.com" type="email" />
            </FieldContent>
            <FieldDescription>We will never share your email.</FieldDescription>
        </Field>
    ),
}

export const Responsive: Story = {
    render: () => (
        <FieldGroup>
            <Field orientation="responsive">
                <FieldLabel>Username</FieldLabel>
                <FieldContent>
                    <Input placeholder="jdoe" />
                </FieldContent>
                <FieldDescription>Your public display name.</FieldDescription>
            </Field>
            <Field orientation="responsive">
                <FieldLabel>Website</FieldLabel>
                <FieldContent>
                    <Input placeholder="https://example.com" type="url" />
                </FieldContent>
            </Field>
        </FieldGroup>
    ),
}

export const WithError: Story = {
    render: () => (
        <Field orientation="vertical">
            <FieldLabel>Password</FieldLabel>
            <FieldContent>
                <Input type="password" placeholder="••••••••" />
            </FieldContent>
            <FieldError errors={[{ message: "Password must be at least 8 characters." }]} />
        </Field>
    ),
}

export const WithFieldSet: Story = {
    render: () => (
        <FieldSet>
            <FieldLegend>Personal Info</FieldLegend>
            <FieldGroup>
                <Field orientation="vertical">
                    <FieldLabel>First Name</FieldLabel>
                    <FieldContent>
                        <Input placeholder="John" />
                    </FieldContent>
                </Field>
                <FieldSeparator />
                <Field orientation="vertical">
                    <FieldLabel>Last Name</FieldLabel>
                    <FieldContent>
                        <Input placeholder="Doe" />
                    </FieldContent>
                </Field>
            </FieldGroup>
        </FieldSet>
    ),
}

export const WithFieldTitle: Story = {
    render: () => (
        <Field orientation="vertical">
            <FieldTitle>API Key</FieldTitle>
            <FieldContent>
                <Input placeholder="sk-..." />
            </FieldContent>
            <FieldDescription>Keep this secret. Never share it publicly.</FieldDescription>
        </Field>
    ),
}
