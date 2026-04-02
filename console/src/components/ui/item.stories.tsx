import type { Meta, StoryObj } from "@storybook/react"
import { Settings, User, Bell } from "lucide-react"
import {
    Item,
    ItemMedia,
    ItemContent,
    ItemActions,
    ItemGroup,
    ItemSeparator,
    ItemTitle,
    ItemDescription,
    ItemHeader,
    ItemFooter,
} from "./item"
import { Button } from "./button"

const meta: Meta<typeof Item> = {
    title: "UI/Item",
    component: Item,
    tags: ["autodocs"],
}

export default meta
type Story = StoryObj<typeof Item>

export const Default: Story = {
    render: () => (
        <Item variant="default">
            <ItemMedia variant="icon">
                <User />
            </ItemMedia>
            <ItemContent>
                <ItemTitle>John Doe</ItemTitle>
                <ItemDescription>Administrator account with full access.</ItemDescription>
            </ItemContent>
            <ItemActions>
                <Button size="sm">Manage</Button>
            </ItemActions>
        </Item>
    ),
}

export const Outline: Story = {
    render: () => (
        <Item variant="outline">
            <ItemMedia variant="icon">
                <Settings />
            </ItemMedia>
            <ItemContent>
                <ItemTitle>Settings</ItemTitle>
                <ItemDescription>Configure your application preferences.</ItemDescription>
            </ItemContent>
            <ItemActions>
                <Button size="sm" variant="outline">
                    Edit
                </Button>
            </ItemActions>
        </Item>
    ),
}

export const Muted: Story = {
    render: () => (
        <Item variant="muted">
            <ItemMedia variant="icon">
                <Bell />
            </ItemMedia>
            <ItemContent>
                <ItemTitle>Notifications</ItemTitle>
                <ItemDescription>Manage your notification preferences.</ItemDescription>
            </ItemContent>
        </Item>
    ),
}

export const Small: Story = {
    render: () => (
        <Item variant="outline" size="sm">
            <ItemMedia variant="icon">
                <User />
            </ItemMedia>
            <ItemContent>
                <ItemTitle>Compact Item</ItemTitle>
                <ItemDescription>A smaller size variant.</ItemDescription>
            </ItemContent>
            <ItemActions>
                <Button size="sm" variant="ghost">
                    View
                </Button>
            </ItemActions>
        </Item>
    ),
}

export const ItemGroupExample: Story = {
    render: () => (
        <ItemGroup>
            <Item variant="outline">
                <ItemMedia variant="icon">
                    <User />
                </ItemMedia>
                <ItemContent>
                    <ItemTitle>Alice Smith</ItemTitle>
                    <ItemDescription>alice@example.com</ItemDescription>
                </ItemContent>
                <ItemActions>
                    <Button size="sm">View</Button>
                </ItemActions>
            </Item>
            <ItemSeparator />
            <Item variant="outline">
                <ItemMedia variant="icon">
                    <User />
                </ItemMedia>
                <ItemContent>
                    <ItemTitle>Bob Jones</ItemTitle>
                    <ItemDescription>bob@example.com</ItemDescription>
                </ItemContent>
                <ItemActions>
                    <Button size="sm">View</Button>
                </ItemActions>
            </Item>
            <ItemSeparator />
            <Item variant="outline">
                <ItemMedia variant="icon">
                    <User />
                </ItemMedia>
                <ItemContent>
                    <ItemTitle>Carol White</ItemTitle>
                    <ItemDescription>carol@example.com</ItemDescription>
                </ItemContent>
                <ItemActions>
                    <Button size="sm">View</Button>
                </ItemActions>
            </Item>
        </ItemGroup>
    ),
}

export const WithHeaderAndFooter: Story = {
    render: () => (
        <Item variant="outline">
            <ItemHeader>
                <ItemTitle>Project Alpha</ItemTitle>
                <span className="text-xs text-muted-foreground">Active</span>
            </ItemHeader>
            <ItemContent>
                <ItemDescription>
                    A long-running project with multiple contributors and ongoing deployments.
                </ItemDescription>
            </ItemContent>
            <ItemFooter>
                <span className="text-xs text-muted-foreground">Last updated 2 hours ago</span>
                <Button size="sm" variant="ghost">
                    Details
                </Button>
            </ItemFooter>
        </Item>
    ),
}
