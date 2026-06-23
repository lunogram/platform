import type { Client } from "../model"

import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"

// GeneralFields: the name and note a client is recognized by.
export function GeneralFields({
    client,
    set,
}: {
    client: Client
    set: (patch: Partial<Client>) => void
}) {
    return (
        <div className="grid gap-x-6 gap-y-5 sm:grid-cols-2">
            <div className="grid gap-2">
                <Label htmlFor="cl-name" className="inline-flex items-center gap-1">
                    Name <span className="text-red">*</span>
                </Label>
                <Input
                    id="cl-name"
                    value={client.name}
                    placeholder="e.g. Mobile app"
                    onChange={(e) => set({ name: e.target.value })}
                />
            </div>
            <div className="grid gap-2">
                <Label htmlFor="cl-desc">Description</Label>
                <Input
                    id="cl-desc"
                    value={client.description ?? ""}
                    placeholder="What this client is for (optional)"
                    onChange={(e) => set({ description: e.target.value })}
                />
            </div>
        </div>
    )
}
