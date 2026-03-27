import { getRandomColor } from "@/lib/colors"
import type { User } from "@/types"
import {
    getUserDisplayName,
    getUserInitials,
    getUserSubtext,
    getPrimaryExternalId,
} from "./userUtils"

function getUserColorSeed(user?: User): string {
    if (!user) return "unknown"
    return user.email ?? getPrimaryExternalId(user as unknown as Record<string, unknown>) ?? user.id
}

interface UserCellProps {
    user?: User
}

export function UserCell({ user }: UserCellProps) {
    const subtext = getUserSubtext(user)

    return (
        <div className="flex items-center gap-3 py-0.5">
            <div
                className="flex h-8 w-8 items-center justify-center rounded-full text-white text-xs font-medium shrink-0"
                style={{ backgroundColor: getRandomColor(getUserColorSeed(user)) }}
            >
                {getUserInitials(user)}
            </div>
            <div className="min-w-0">
                <div className="font-medium text-sm truncate">{getUserDisplayName(user)}</div>
                {subtext && <div className="text-xs text-muted-foreground truncate">{subtext}</div>}
            </div>
        </div>
    )
}
