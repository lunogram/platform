import { useTranslation } from "react-i18next"
import { useForm } from "react-hook-form"
import { Check, Eye, Smartphone, Pencil, Shield } from "lucide-react"
import { snakeToTitle } from "../../utils"
import type { ProjectRole } from "../../types"
import { projectRoles } from "../../types"

const roleHierarchy: Record<string, number> = {
    support: 1,
    client: 1,
    editor: 2,
    admin: 3,
}

function getAllowedRoles(userRole: ProjectRole): ProjectRole[] {
    const userLevel = roleHierarchy[userRole] ?? 0
    return projectRoles.filter(
        (role) => projectRoles.includes(role) && (roleHierarchy[role] ?? 0) <= userLevel,
    )
}

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog"

const roleConfig: Record<
    ProjectRole,
    {
        icon: typeof Eye
        description: string
        permissions: string[]
    }
> = {
    support: {
        icon: Eye,
        description: "Read-only access across all project resources.",
        permissions: [
            "View users, campaigns, journeys, templates",
            "View events, lists, tags, and documents",
            "No create, update, or delete access",
        ],
    },
    client: {
        icon: Smartphone,
        description: "Write-only access for client-side SDKs and integrations.",
        permissions: [
            "Create & update users, events, and organizations",
            "No read access to any resource",
            "Designed for client-side SDKs and public integrations",
        ],
    },
    editor: {
        icon: Pencil,
        description: "Full content management for campaigns, journeys, and more.",
        permissions: [
            "Everything in Viewer",
            "Create & manage campaigns, templates, journeys",
            "Manage lists, tags, documents, locales, and actions",
        ],
    },
    admin: {
        icon: Shield,
        description: "Full project access including providers and dangerous operations.",
        permissions: [
            "Everything in Editor",
            "Manage providers and integrations",
            "Delete subscriptions, actions, and organizations",
        ],
    },
}

interface InviteFormData {
    email: string
    role: ProjectRole
    expires_in?: string
}

export interface InviteDialogProps {
    isOpen: boolean
    onClose: () => void
    onSave: (data: InviteFormData) => Promise<void>
    isSaving: boolean
    userRole?: ProjectRole
}

export default function InviteDialog({
    isOpen,
    onClose,
    onSave,
    isSaving,
    userRole = "support",
}: InviteDialogProps) {
    const { t } = useTranslation()
    const allowedRoles = getAllowedRoles(userRole)
    const form = useForm<InviteFormData>({
        values: {
            role: allowedRoles.includes("admin") ? "admin" : allowedRoles[0] || "support",
            email: "",
            expires_in: "24h",
        },
    })

    // eslint-disable-next-line react-hooks/incompatible-library
    const selectedRole = form.watch("role") ?? allowedRoles[0]
    const expiresIn = form.watch("expires_in") ?? "24h"

    const expiryOptions = [
        { value: "1h", label: "1 hour" },
        { value: "24h", label: "24 hours" },
        { value: "48h", label: "48 hours" },
        { value: "7d", label: "7 days" },
        { value: "30d", label: "30 days" },
    ]

    const validRoles = allowedRoles

    return (
        <Dialog
            open={isOpen}
            onOpenChange={(open) => {
                if (!open) onClose()
            }}
        >
            <DialogContent className="sm:max-w-xl">
                <DialogHeader>
                    <DialogTitle>{t("create_invite")}</DialogTitle>
                    <DialogDescription>
                        {t("create_invite_description", "Invite a team member to your project.")}
                    </DialogDescription>
                </DialogHeader>

                <form onSubmit={form.handleSubmit(onSave)} className="grid gap-4 py-2">
                    <div className="grid gap-2">
                        <Label htmlFor="email" className="inline-flex items-center gap-1">
                            {t("email")} <span className="text-destructive">*</span>
                        </Label>
                        <Input
                            id="email"
                            type="email"
                            placeholder="colleague@example.com"
                            {...form.register("email", { required: true })}
                        />
                    </div>

                    <div className="grid gap-2">
                        <Label htmlFor="expires_in" className="inline-flex items-center gap-1">
                            {t("expires_in", "Expires in")}
                        </Label>
                        <select
                            id="expires_in"
                            className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
                            {...form.register("expires_in")}
                            value={expiresIn}
                            onChange={(e) => form.setValue("expires_in", e.target.value)}
                        >
                            {expiryOptions.map((option) => (
                                <option key={option.value} value={option.value}>
                                    {option.label}
                                </option>
                            ))}
                        </select>
                    </div>

                    <div className="grid gap-2">
                        <Label className="inline-flex items-center gap-1">
                            {t("role")} <span className="text-destructive">*</span>
                        </Label>
                        <div className="grid grid-cols-2 gap-2">
                            {validRoles.map((role) => {
                                const config = roleConfig[role]
                                const Icon = config.icon
                                const isSelected = selectedRole === role

                                return (
                                    <button
                                        key={role}
                                        type="button"
                                        onClick={() => form.setValue("role", role)}
                                        className={`relative flex flex-col gap-1.5 rounded-lg border p-3 text-left text-sm transition-colors hover:bg-accent/50 ${
                                            isSelected
                                                ? "border-primary bg-primary/5 ring-1 ring-primary"
                                                : "border-border"
                                        }`}
                                    >
                                        {isSelected && (
                                            <div className="absolute right-2 top-2">
                                                <Check className="h-3.5 w-3.5 text-primary" />
                                            </div>
                                        )}
                                        <div className="flex items-center gap-2">
                                            <Icon
                                                className={`h-4 w-4 ${isSelected ? "text-primary" : "text-muted-foreground"}`}
                                            />
                                            <span className="font-medium">
                                                {snakeToTitle(role)}
                                            </span>
                                        </div>
                                        <p className="text-xs text-muted-foreground leading-snug pr-4">
                                            {config.description}
                                        </p>
                                    </button>
                                )
                            })}
                        </div>

                        <div className="rounded-lg border bg-muted/30 p-3 mt-1">
                            <p className="text-xs font-medium text-muted-foreground mb-2">
                                {snakeToTitle(selectedRole)} {t("permissions", "permissions")}
                            </p>
                            <ul className="space-y-1">
                                {roleConfig[selectedRole]?.permissions.map((perm) => (
                                    <li
                                        key={perm}
                                        className="flex items-start gap-2 text-xs text-muted-foreground"
                                    >
                                        <Check className="h-3 w-3 mt-0.5 shrink-0 text-primary" />
                                        <span>{perm}</span>
                                    </li>
                                ))}
                            </ul>
                        </div>
                    </div>

                    <DialogFooter className="pt-2">
                        <Button
                            type="button"
                            variant="outline"
                            onClick={onClose}
                            disabled={isSaving}
                        >
                            {t("cancel")}
                        </Button>
                        <Button type="submit" disabled={isSaving}>
                            {isSaving ? t("saving", "Saving...") : t("create_invite")}
                        </Button>
                    </DialogFooter>
                </form>
            </DialogContent>
        </Dialog>
    )
}
