import { ScanLine } from "lucide-react"

import { grantsFor, hasVerifiedSubject, type Client } from "../model"
import type { PermissionSelection } from "../../PermissionSelector"
import PermissionSelector from "../../PermissionSelector"
import { describePermissions } from "@/lib/rbac-presets"

// PermissionsFields: a plain-English summary of what the client can do, then the
// preset cards and the resource × verb matrix to refine it.
export function PermissionsFields({
    client,
    set,
}: {
    client: Client
    set: (patch: Partial<Client>) => void
}) {
    const ownScope = client.subjectScope === "own" && hasVerifiedSubject(client.identity)
    const summary = describePermissions(grantsFor(client.permissions), ownScope)

    return (
        <div className="flex flex-col gap-5">
            <div className="flex items-start gap-2.5 rounded-md bg-surface-soft px-3.5 py-3 text-sm">
                <ScanLine className="mt-px h-4 w-4 shrink-0 text-ink-soft" strokeWidth={1.75} />
                <span className="text-foreground">{summary}</span>
            </div>
            <PermissionSelector
                selection={client.permissions}
                onChange={(permissions: PermissionSelection) => set({ permissions })}
            />
        </div>
    )
}
