import type { components } from "@/oapi/management.generated"
import type { AdminOrganization, Project } from "@/types"

type Campaign = components["schemas"]["Campaign"]

export type CampaignRow = Pick<Campaign, "id" | "name" | "channel" | "delivery" | "updated_at">

export const organizations: AdminOrganization[] = [
    {
        id: "0b6f8d0e-6f4f-4d3e-9a51-0c6f1f6a0001",
        name: "Northwind",
        role: "owner",
        is_active: true,
    },
    {
        id: "0b6f8d0e-6f4f-4d3e-9a51-0c6f1f6a0002",
        name: "Contoso",
        role: "member",
        is_active: false,
    },
]

export const project: Project = {
    id: "5d9a3c1e-2b7f-4e8a-9c0d-1f2e3a4b5c6d",
    name: "Northwind Mobile",
    locale: "en",
    timezone: "UTC",
    created_at: "2026-01-12T09:00:00Z",
    updated_at: "2026-11-02T16:45:00Z",
    role: "admin",
    has_provider: true,
}

export const projects: Project[] = [
    project,
    {
        ...project,
        id: "5d9a3c1e-2b7f-4e8a-9c0d-1f2e3a4b5c6e",
        name: "Northwind Web",
    },
]

function delivery(sent: number, total: number): CampaignRow["delivery"] {
    return { sent, total, opens: Math.round(sent * 0.42), clicks: Math.round(sent * 0.08) }
}

export const campaigns: CampaignRow[] = [
    {
        id: "c1a2b3c4-0001-4000-8000-000000000001",
        name: "Welcome series",
        channel: "email",
        delivery: delivery(48210, 49875),
        updated_at: "2026-11-14T10:24:00Z",
    },
    {
        id: "c1a2b3c4-0001-4000-8000-000000000002",
        name: "Order shipped",
        channel: "sms",
        delivery: delivery(12034, 12034),
        updated_at: "2026-11-12T08:02:00Z",
    },
    {
        id: "c1a2b3c4-0001-4000-8000-000000000003",
        name: "Cart reminder",
        channel: "push",
        delivery: delivery(7311, 9120),
        updated_at: "2026-11-09T17:40:00Z",
    },
    {
        id: "c1a2b3c4-0001-4000-8000-000000000004",
        name: "Product updates",
        channel: "inbox",
        delivery: delivery(2290, 2500),
        updated_at: "2026-11-03T12:15:00Z",
    },
    {
        id: "c1a2b3c4-0001-4000-8000-000000000005",
        name: "Holiday preview",
        channel: "email",
        delivery: delivery(0, 0),
        updated_at: "2026-10-28T14:55:00Z",
    },
]
