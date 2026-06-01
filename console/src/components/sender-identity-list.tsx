import { useCallback, useState } from "react"
import { useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { Loader2, Trash2 } from "lucide-react"
import * as z from "zod"

import oapiClient, { type SenderIdentity } from "@/oapi/client"
import { useResolver } from "@/hooks"
import { phoneSchema } from "@/validation/phone"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Badge } from "@/components/ui/badge"
import { Skeleton } from "@/components/ui/skeleton"
import { Separator } from "@/components/ui/separator"
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from "@/components/ui/table"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"

interface SenderIdentityListProps {
    projectId: string
    providerId: string
    channel: "email" | "sms"
    /** The currently selected default_from identity ID */
    defaultFromId?: string
    /** Called when the user marks an identity as the default */
    onDefaultChange?: (identityId: string) => void
}

export function SenderIdentityList({
    projectId,
    providerId,
    channel,
    defaultFromId,
    onDefaultChange,
}: SenderIdentityListProps) {
    const { t } = useTranslation()
    const [isCreating, setIsCreating] = useState(false)
    const [createError, setCreateError] = useState<string | null>(null)
    const [hasAttempted, setHasAttempted] = useState(false)
    const [confirmDeleteId, setConfirmDeleteId] = useState<string | null>(null)

    const addressForm = useForm<{ address: string; name: string }>({
        defaultValues: { address: "", name: "" },
    })
    const newAddress = addressForm.watch("address")
    const newName = addressForm.watch("name")
    const [isDeleting, setIsDeleting] = useState(false)

    const [result, , reload] = useResolver(
        useCallback(async () => {
            const { data, error } = await oapiClient.GET(
                "/api/admin/projects/{projectID}/sender-identities",
                {
                    params: {
                        path: { projectID: projectId },
                        query: { provider_id: providerId, channel },
                    },
                },
            )
            if (error) throw error
            return data?.results ?? []
        }, [projectId, providerId, channel]),
    )

    const identities: SenderIdentity[] | undefined = result ?? undefined

    const isValidAddress = (address: string) => {
        if (!address.trim()) return false
        return channel === "email"
            ? z.email().safeParse(address.trim()).success
            : phoneSchema.safeParse(address.trim()).success
    }

    const inputIsValid = isValidAddress(newAddress)
    const showValidationError = hasAttempted && newAddress.trim() && !inputIsValid

    const handleCreate = async () => {
        setHasAttempted(true)
        const address = newAddress.trim()
        if (!address || !isValidAddress(address)) return
        setIsCreating(true)
        setCreateError(null)
        try {
            const traits: Record<string, unknown> = { address }
            if (channel === "email" && newName.trim()) {
                traits.name = newName.trim()
            }
            const { error } = await oapiClient.POST(
                "/api/admin/projects/{projectID}/sender-identities",
                {
                    params: { path: { projectID: projectId } },
                    body: { provider_id: providerId, channel, traits },
                },
            )
            if (error) {
                setCreateError(
                    t("sender_identity_create_error", "This address has already been added."),
                )
                return
            }
            addressForm.reset()
            setHasAttempted(false)
            await reload()
        } catch {
            setCreateError(t("sender_identity_create_error_generic", "Failed to add address."))
        } finally {
            setIsCreating(false)
        }
    }

    const handleDelete = async (identity: SenderIdentity) => {
        setIsDeleting(true)
        try {
            await oapiClient.DELETE(
                "/api/admin/projects/{projectID}/sender-identities/{senderIdentityID}",
                {
                    params: {
                        path: {
                            projectID: projectId,
                            senderIdentityID: identity.id,
                        },
                    },
                },
            )
            setConfirmDeleteId(null)
            await reload()
        } catch (err) {
            console.error("Failed to delete sender identity:", err)
        } finally {
            setIsDeleting(false)
        }
    }

    const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
        if (e.key === "Enter") {
            e.preventDefault()
            handleCreate()
        }
    }

    return (
        <Card className="shadow-sm">
            {/* Top zone — header + input */}
            <CardHeader className="pb-4">
                <CardTitle>
                    {channel === "email"
                        ? t("sender_addresses", "Sender addresses")
                        : t("sender_numbers", "Sender numbers")}
                </CardTitle>
                <CardDescription>
                    {channel === "email"
                        ? t(
                              "sender_addresses_email_description",
                              "Email addresses that can be used as the sender for this integration.",
                          )
                        : t(
                              "sender_addresses_sms_description",
                              "Phone numbers that can be used as the sender for this integration.",
                          )}
                </CardDescription>
            </CardHeader>
            <CardContent className="pb-5">
                <div className="grid gap-1.5">
                    <div className="flex gap-2 -mx-3">
                        {channel === "email" && (
                            <Input
                                type="text"
                                className="shadow-none"
                                {...addressForm.register("name")}
                                onKeyDown={handleKeyDown}
                                placeholder="Display name"
                                disabled={isCreating}
                            />
                        )}
                        <Input
                            type={channel === "email" ? "email" : "tel"}
                            className="shadow-none"
                            {...addressForm.register("address")}
                            onChange={(e) => {
                                addressForm.setValue("address", e.target.value)
                                if (createError) setCreateError(null)
                            }}
                            onBlur={() => {
                                if (newAddress.trim()) setHasAttempted(true)
                            }}
                            onKeyDown={handleKeyDown}
                            placeholder={channel === "email" ? "name@company.com" : "+1234567890"}
                            disabled={isCreating}
                        />
                        <Button
                            type="button"
                            variant="outline"
                            size="sm"
                            className="h-9 shrink-0 shadow-none"
                            onClick={addressForm.handleSubmit(handleCreate)}
                            disabled={!newAddress.trim() || !inputIsValid || isCreating}
                        >
                            {isCreating && <Loader2 className="h-4 w-4 animate-spin" />}
                            {t("add", "Add")}
                        </Button>
                    </div>
                    {showValidationError && (
                        <p className="text-sm text-destructive">
                            {channel === "email"
                                ? t("invalid_email", "Please enter a valid email address.")
                                : t("invalid_phone", "Please enter a valid phone number.")}
                        </p>
                    )}
                    {createError && <p className="text-sm text-destructive">{createError}</p>}
                </div>
            </CardContent>

            {/* Zone divider */}
            <Separator />

            {/* Bottom zone — table or empty state */}
            <div className="bg-muted/50 rounded-b-xl">
                {identities === undefined ? (
                    <div className="px-6 py-6">
                        <div className="space-y-3">
                            {Array.from({ length: 2 }).map((_, i) => (
                                <div key={i} className="flex items-center gap-3">
                                    <Skeleton className="h-3.5 w-44" />
                                </div>
                            ))}
                        </div>
                    </div>
                ) : identities.length === 0 ? (
                    <div className="py-8 px-4 text-center">
                        <p className="text-sm text-muted-foreground">
                            {channel === "email"
                                ? t("no_sender_addresses", "No sender addresses configured yet.")
                                : t("no_sender_numbers", "No sender numbers configured yet.")}
                        </p>
                        <p className="text-sm text-muted-foreground mt-1">
                            {channel === "email"
                                ? t(
                                      "no_sender_addresses_hint",
                                      "Add one above to start sending emails.",
                                  )
                                : t(
                                      "no_sender_numbers_hint",
                                      "Add one above to start sending messages.",
                                  )}
                        </p>
                    </div>
                ) : (
                    <Table>
                        <TableHeader>
                            <TableRow className="hover:bg-transparent">
                                <TableHead className="first:pl-6 last:pr-6">
                                    {channel === "email"
                                        ? t("address", "Address")
                                        : t("number", "Number")}
                                </TableHead>
                                <TableHead className="w-0 first:pl-6 last:pr-6" />
                            </TableRow>
                        </TableHeader>
                        <TableBody>
                            {identities.map((identity) => {
                                const isDefault = identity.id === defaultFromId
                                const isConfirming = confirmDeleteId === identity.id

                                if (isConfirming) {
                                    return (
                                        <TableRow
                                            key={identity.id}
                                            className="hover:bg-transparent"
                                        >
                                            <TableCell className="first:pl-6 last:pr-6">
                                                <span className="text-sm text-muted-foreground">
                                                    {t("remove_confirm", "Remove {{address}}?", {
                                                        address:
                                                            (identity.traits?.address as string) ??
                                                            "",
                                                    })}
                                                </span>
                                            </TableCell>
                                            <TableCell className="text-right first:pl-6 last:pr-6">
                                                <div className="flex items-center justify-end gap-1">
                                                    <Button
                                                        type="button"
                                                        variant="ghost"
                                                        size="sm"
                                                        onClick={() => setConfirmDeleteId(null)}
                                                        disabled={isDeleting}
                                                    >
                                                        {t("cancel", "Cancel")}
                                                    </Button>
                                                    <Button
                                                        type="button"
                                                        variant="ghost"
                                                        size="sm"
                                                        className="text-destructive hover:text-destructive"
                                                        onClick={() => handleDelete(identity)}
                                                        disabled={isDeleting}
                                                    >
                                                        {isDeleting && (
                                                            <Loader2 className="h-3.5 w-3.5 animate-spin" />
                                                        )}
                                                        {t("remove", "Remove")}
                                                    </Button>
                                                </div>
                                            </TableCell>
                                        </TableRow>
                                    )
                                }

                                return (
                                    <TableRow
                                        key={identity.id}
                                        className="group hover:bg-transparent"
                                    >
                                        <TableCell className="first:pl-6 last:pr-6">
                                            <div className="flex items-center gap-2.5">
                                                {channel === "email" && identity.traits?.name ? (
                                                    <div className="flex flex-col">
                                                        <span className="text-sm text-foreground">
                                                            {identity.traits.name as string}
                                                        </span>
                                                        <span className="text-xs font-mono text-muted-foreground">
                                                            {identity.traits?.address as string}
                                                        </span>
                                                    </div>
                                                ) : (
                                                    <span className="text-sm font-mono text-foreground">
                                                        {identity.traits?.address as string}
                                                    </span>
                                                )}
                                                {isDefault && (
                                                    <Badge
                                                        variant="secondary"
                                                        className="shrink-0 text-xs font-normal"
                                                    >
                                                        {t("default", "Default")}
                                                    </Badge>
                                                )}
                                            </div>
                                        </TableCell>
                                        <TableCell className="text-right first:pl-6 last:pr-6">
                                            <div className="flex items-center justify-end gap-1 opacity-0 group-hover:opacity-100 focus-within:opacity-100 transition-opacity">
                                                {!isDefault && onDefaultChange && (
                                                    <Button
                                                        variant="ghost"
                                                        size="sm"
                                                        className="h-8 text-xs text-muted-foreground"
                                                        onClick={() => onDefaultChange(identity.id)}
                                                        type="button"
                                                    >
                                                        {t("set_as_default", "Set as default")}
                                                    </Button>
                                                )}
                                                <Button
                                                    variant="ghost"
                                                    size="icon"
                                                    className="h-8 w-8 text-muted-foreground hover:text-destructive"
                                                    onClick={() => setConfirmDeleteId(identity.id)}
                                                    type="button"
                                                >
                                                    <Trash2 className="h-3.5 w-3.5" />
                                                </Button>
                                            </div>
                                        </TableCell>
                                    </TableRow>
                                )
                            })}
                        </TableBody>
                    </Table>
                )}
            </div>
        </Card>
    )
}
