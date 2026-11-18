import type { Meta, StoryObj } from "@storybook/react-vite"
import { ChevronRight, Mail, Smartphone } from "lucide-react"

import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
    Item,
    ItemActions,
    ItemContent,
    ItemDescription,
    ItemGroup,
    ItemMedia,
    ItemSeparator,
    ItemTitle,
} from "@/components/ui/item"
import { JsonInline, JsonView } from "@/components/ui/json-view"

const meta: Meta = {
    title: "UI/Data Display",
}

export default meta
type Story = StoryObj

// Inline so the story never fetches an image.
const avatarSrc =
    "data:image/svg+xml;utf8," +
    encodeURIComponent(
        '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 40 40"><rect width="40" height="40" fill="#d1e0ff"/><circle cx="20" cy="16" r="7" fill="#2970ff"/><rect x="8" y="26" width="24" height="14" rx="7" fill="#2970ff"/></svg>',
    )

export const Avatars: Story = {
    render: () => (
        <div className="flex items-center gap-3">
            <Avatar>
                <AvatarImage src={avatarSrc} alt="Jane Cooper" />
                <AvatarFallback>JC</AvatarFallback>
            </Avatar>
            <Avatar>
                <AvatarFallback>WW</AvatarFallback>
            </Avatar>
            <Avatar className="h-8 w-8 rounded-lg">
                <AvatarFallback className="rounded-lg text-xs">EH</AvatarFallback>
            </Avatar>
        </div>
    ),
}

export const Items: Story = {
    render: () => (
        <div className="flex max-w-md flex-col gap-4">
            <Item variant="outline">
                <ItemMedia variant="icon">
                    <Mail />
                </ItemMedia>
                <ItemContent>
                    <ItemTitle>
                        Resend <Badge variant="secondary">Default</Badge>
                    </ItemTitle>
                    <ItemDescription>
                        Email provider · sends from hello@northwind.example
                    </ItemDescription>
                </ItemContent>
                <ItemActions>
                    <Button variant="outline" size="sm">
                        Configure
                    </Button>
                </ItemActions>
            </Item>
            <Item variant="muted" size="sm">
                <ItemContent>
                    <ItemTitle>No SMS provider</ItemTitle>
                    <ItemDescription>Connect Twilio to send text messages.</ItemDescription>
                </ItemContent>
                <ItemActions>
                    <Button size="sm">Connect</Button>
                </ItemActions>
            </Item>
            <ItemGroup className="rounded-md border">
                {[
                    { name: "Jane Cooper", email: "jane@example.com" },
                    { name: "Wade Warren", email: "wade@example.com" },
                ].map((user, i) => (
                    <div key={user.email}>
                        {i > 0 && <ItemSeparator />}
                        <Item size="sm" asChild>
                            <a href="#">
                                <ItemMedia>
                                    <Avatar className="h-8 w-8">
                                        <AvatarFallback className="text-xs">
                                            {user.name
                                                .split(" ")
                                                .map((part) => part[0])
                                                .join("")}
                                        </AvatarFallback>
                                    </Avatar>
                                </ItemMedia>
                                <ItemContent>
                                    <ItemTitle>{user.name}</ItemTitle>
                                    <ItemDescription>{user.email}</ItemDescription>
                                </ItemContent>
                                <ItemActions>
                                    <ChevronRight className="h-4 w-4 text-muted-foreground" />
                                </ItemActions>
                            </a>
                        </Item>
                    </div>
                ))}
            </ItemGroup>
            <Item variant="outline" size="sm">
                <ItemMedia variant="icon">
                    <Smartphone />
                </ItemMedia>
                <ItemContent>
                    <ItemTitle>Push</ItemTitle>
                </ItemContent>
            </Item>
        </div>
    ),
}

const event = {
    name: "order_completed",
    external_id: "usr_01",
    data: {
        order_id: "ord_7731",
        total: 128.4,
        currency: "USD",
        gift: false,
        coupon: null,
        items: [
            { sku: "TEE-BLK-M", quantity: 2 },
            { sku: "CAP-NVY", quantity: 1 },
        ],
    },
}

export const Json: Story = {
    render: () => (
        <div className="flex max-w-xl flex-col gap-6">
            <JsonView data={event} title="Event payload" />
            <JsonView data={event} title="Collapsed" defaultExpanded={false} />
            <JsonView data={{}} title="Empty object" />
            <div className="text-sm">
                <JsonInline data={event.data} maxLength={60} />
            </div>
        </div>
    ),
}
