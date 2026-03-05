import type { ComponentType } from "react"

import { ClerkUserDropdown } from "./clerk"
import { DefaultUserDropdown } from "./default"
import type { UserDropdownProps } from "./types"

interface AuthProvider {
    isConfigured: () => boolean
    Component: ComponentType<UserDropdownProps>
}

const providers: Record<string, AuthProvider> = {
    clerk: {
        isConfigured: () => Boolean(import.meta.env.VITE_CLERK_PUBLISHABLE_KEY),
        Component: ClerkUserDropdown,
    },
    default: {
        isConfigured: () => true,
        Component: DefaultUserDropdown,
    },
}

function resolveUserDropdown(): ComponentType<UserDropdownProps> {
    for (const provider of Object.values(providers)) {
        if (provider.isConfigured()) {
            return provider.Component
        }
    }
    return DefaultUserDropdown
}

export const UserDropdown = resolveUserDropdown()
export type { UserDropdownProps }
