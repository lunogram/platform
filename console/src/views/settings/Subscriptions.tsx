import { useCallback, useContext, useMemo, useState } from "react"
import { useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { Plus, Search, Bell, MoreHorizontal } from "lucide-react"
import api from "../../api"
import { ProjectContext } from "../../contexts"
import { useResolver } from "../../hooks"
import { snakeToTitle } from "../../utils"
import type { Subscription } from "../../types"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from "@/components/ui/table"
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog"
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select"
import { Skeleton } from "@/components/ui/skeleton"
import { Badge } from "@/components/ui/badge"

export default function Subscriptions() {
    const { t } = useTranslation()
    const [project] = useContext(ProjectContext)

    const [searchQuery, setSearchQuery] = useState("")
    const [editing, setEditing] = useState<null | Partial<Subscription>>(null)

    const [result, , reload] = useResolver(
        useCallback(async () => {
            return await api.subscriptions.search(project.id, { limit: 50 })
        }, [project.id]),
    )

    const subscriptions = useMemo(() => {
        const all = result?.results ?? []
        if (!searchQuery) return all
        const query = searchQuery.toLowerCase()
        return all.filter((sub) => sub.name.toLowerCase().includes(query))
    }, [result, searchQuery])

    return (
        <div className="flex flex-col gap-6">
            {/* Header */}
            <h2 className="text-2xl font-semibold tracking-tight">{t("subscriptions")}</h2>

            {/* Search and Actions */}
            <div className="flex items-center justify-between gap-4">
                <div className="relative max-w-sm flex-1">
                    <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                    <Input
                        placeholder={t("search")}
                        value={searchQuery}
                        onChange={(e) => setSearchQuery(e.target.value)}
                        className="pl-9"
                    />
                </div>
                <Button onClick={() => setEditing({ channel: "email" })}>
                    <Plus className="mr-2 h-4 w-4" />
                    {t("create_subscription")}
                </Button>
            </div>

            {/* Table */}
            <div className="rounded-lg border bg-card">
                <Table>
                    <TableHeader>
                        <TableRow>
                            <TableHead>{t("name")}</TableHead>
                            <TableHead>{t("channel")}</TableHead>
                            <TableHead>{t("public")}</TableHead>
                            <TableHead className="w-[70px]" />
                        </TableRow>
                    </TableHeader>
                    <TableBody>
                        {!result ? (
                            Array.from({ length: 3 }).map((_, i) => (
                                <TableRow key={i}>
                                    <TableCell>
                                        <Skeleton className="h-4 w-28" />
                                    </TableCell>
                                    <TableCell>
                                        <Skeleton className="h-4 w-16" />
                                    </TableCell>
                                    <TableCell>
                                        <Skeleton className="h-4 w-10" />
                                    </TableCell>
                                    <TableCell>
                                        <Skeleton className="h-4 w-8" />
                                    </TableCell>
                                </TableRow>
                            ))
                        ) : subscriptions.length === 0 ? (
                            <TableRow>
                                <TableCell colSpan={4} className="h-32 text-center">
                                    <div className="flex flex-col items-center gap-2 text-muted-foreground">
                                        <Bell className="h-8 w-8" />
                                        <p>
                                            {searchQuery
                                                ? t("no_results")
                                                : t("no_subscriptions_yet", "No subscriptions yet")}
                                        </p>
                                        {!searchQuery && (
                                            <Button
                                                variant="outline"
                                                size="sm"
                                                onClick={() => setEditing({ channel: "email" })}
                                                className="mt-2"
                                            >
                                                <Plus className="mr-2 h-4 w-4" />
                                                {t("create_subscription")}
                                            </Button>
                                        )}
                                    </div>
                                </TableCell>
                            </TableRow>
                        ) : (
                            subscriptions.map((sub) => (
                                <TableRow
                                    key={sub.id}
                                    className="cursor-pointer"
                                    onClick={() => setEditing(sub)}
                                >
                                    <TableCell className="font-medium">{sub.name}</TableCell>
                                    <TableCell>
                                        <Badge variant="secondary">
                                            {snakeToTitle(sub.channel)}
                                        </Badge>
                                    </TableCell>
                                    <TableCell className="text-muted-foreground">
                                        {sub.is_public ? t("yes") : t("no")}
                                    </TableCell>
                                    <TableCell>
                                        <DropdownMenu>
                                            <DropdownMenuTrigger asChild>
                                                <Button
                                                    variant="ghost"
                                                    className="h-8 w-8 p-0"
                                                    onClick={(e) => e.stopPropagation()}
                                                    aria-label={t("options")}
                                                >
                                                    <MoreHorizontal className="h-4 w-4" />
                                                </Button>
                                            </DropdownMenuTrigger>
                                            <DropdownMenuContent align="end">
                                                <DropdownMenuItem
                                                    onClick={(e) => {
                                                        e.stopPropagation()
                                                        setEditing(sub)
                                                    }}
                                                >
                                                    {t("edit")}
                                                </DropdownMenuItem>
                                            </DropdownMenuContent>
                                        </DropdownMenu>
                                    </TableCell>
                                </TableRow>
                            ))
                        )}
                    </TableBody>
                </Table>

                {subscriptions.length > 0 && (
                    <div className="flex items-center justify-between border-t px-4 py-3">
                        <p className="text-sm text-muted-foreground">
                            {subscriptions.length}{" "}
                            {subscriptions.length === 1
                                ? t("subscription", "subscription")
                                : t("subscriptions")}
                        </p>
                    </div>
                )}
            </div>

            {/* Create/Edit Subscription Dialog */}
            <SubscriptionDialog
                editing={editing}
                onClose={() => setEditing(null)}
                onSave={async (data) => {
                    const { id, name, channel, is_public } = data
                    if (id) {
                        await api.subscriptions.update(project.id, id, { name, is_public })
                    } else {
                        await api.subscriptions.create(project.id, { name, channel, is_public })
                    }
                    await reload()
                    setEditing(null)
                }}
            />
        </div>
    )
}

interface SubscriptionDialogProps {
    editing: Partial<Subscription> | null
    onClose: () => void
    onSave: (data: Partial<Subscription>) => Promise<void>
}

function SubscriptionDialog({ editing, onClose, onSave }: SubscriptionDialogProps) {
    const { t } = useTranslation()
    const [isSaving, setIsSaving] = useState(false)
    const form = useForm<Partial<Subscription>>({
        values: editing ?? undefined,
    })

    const isUpdate = !!editing?.id

    const handleSubmit = async (data: Partial<Subscription>) => {
        setIsSaving(true)
        try {
            await onSave(data)
        } finally {
            setIsSaving(false)
        }
    }

    return (
        <Dialog
            open={!!editing}
            onOpenChange={(open) => {
                if (!open) onClose()
            }}
        >
            <DialogContent>
                <DialogHeader>
                    <DialogTitle>
                        {isUpdate ? t("update_subscription") : t("create_subscription")}
                    </DialogTitle>
                    <DialogDescription>
                        {isUpdate
                            ? t(
                                  "update_subscription_description",
                                  "Update the subscription details.",
                              )
                            : t(
                                  "create_subscription_description",
                                  "Create a new subscription topic.",
                              )}
                    </DialogDescription>
                </DialogHeader>

                <form onSubmit={form.handleSubmit(handleSubmit)} className="grid gap-4 py-2">
                    <div className="grid gap-2">
                        <Label htmlFor="sub_name">
                            {t("name")} <span className="inline text-destructive">*</span>
                        </Label>
                        <Input id="sub_name" {...form.register("name", { required: true })} />
                    </div>

                    <div className="flex items-center justify-between gap-4 rounded-lg border p-3">
                        <div className="space-y-0.5">
                            <Label>{t("public")}</Label>
                            <p className="text-sm text-muted-foreground">{t("public_desc")}</p>
                        </div>
                        <Switch
                            checked={form.watch("is_public") ?? false}
                            onCheckedChange={(checked) => form.setValue("is_public", checked)}
                        />
                    </div>

                    {!isUpdate && (
                        <div className="grid gap-2">
                            <Label>{t("channel")}</Label>
                            <Select
                                value={form.watch("channel") ?? "email"}
                                onValueChange={(val) =>
                                    form.setValue("channel", val as Subscription["channel"])
                                }
                            >
                                <SelectTrigger>
                                    <SelectValue />
                                </SelectTrigger>
                                <SelectContent>
                                    {["email", "push", "text"].map((channel) => (
                                        <SelectItem key={channel} value={channel}>
                                            {snakeToTitle(channel)}
                                        </SelectItem>
                                    ))}
                                </SelectContent>
                            </Select>
                        </div>
                    )}

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
                            {isSaving
                                ? t("saving", "Saving...")
                                : isUpdate
                                  ? t("update_subscription")
                                  : t("create_subscription")}
                        </Button>
                    </DialogFooter>
                </form>
            </DialogContent>
        </Dialog>
    )
}
