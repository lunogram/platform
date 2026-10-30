import { useContext, useState } from "react"
import { useTranslation } from "react-i18next"
import { useQuery } from "@tanstack/react-query"
import { Bell, BellOff, Link2 } from "lucide-react"
import { toast } from "sonner"
import { ProjectContext, UserContext } from "../../contexts"
import { snakeToTitle } from "../../utils"
import type { SubscriptionState } from "../../types"
import { LoadErrorRow } from "./LoadErrorRow"
import { useUnsubscribeAll, useUpdateUserSubscription, userSubscriptionsQuery } from "./queries"
import type { UUID } from "@/types/common"

import { Button } from "@/components/ui/button"
import { Switch } from "@/components/ui/switch"
import { Skeleton } from "@/components/ui/skeleton"
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

export default function UserDetailSubscriptions() {
    const { t } = useTranslation()
    const [project] = useContext(ProjectContext)
    const [user] = useContext(UserContext)

    const [confirmAction, setConfirmAction] = useState<{
        type: "toggle" | "unsubscribe_all"
        subscriptionId?: UUID
        newState?: SubscriptionState
    } | null>(null)

    const search = useQuery(userSubscriptionsQuery(project.id, user.id))
    const updateSubscription = useUpdateUserSubscription()
    const unsubscribeAll = useUnsubscribeAll()
    const isSaving = updateSubscription.isPending || unsubscribeAll.isPending

    const subscriptions = search.data?.results ?? []

    const handleToggle = (subscriptionId: UUID, newState: SubscriptionState) => {
        setConfirmAction({ type: "toggle", subscriptionId, newState })
    }

    const handleUnsubscribeAll = () => {
        setConfirmAction({ type: "unsubscribe_all" })
    }

    // The dialog stays open on failure; the mutation cache reports the error.
    const executeAction = () => {
        if (!confirmAction) return
        const onSuccess = () => setConfirmAction(null)

        if (confirmAction.type === "toggle" && confirmAction.subscriptionId) {
            updateSubscription.mutate(
                {
                    subscription_id: confirmAction.subscriptionId,
                    state: confirmAction.newState!,
                },
                { onSuccess },
            )
        } else if (confirmAction.type === "unsubscribe_all") {
            unsubscribeAll.mutate(undefined, { onSuccess })
        }
    }

    return (
        <div className="space-y-4">
            {/* Section Header */}
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
                <div>
                    <h2 className="text-base font-medium">{t("subscriptions")}</h2>
                    <p className="text-sm text-muted-foreground mt-0.5">
                        {t(
                            "subscriptions_description",
                            "Manage subscription preferences for this user",
                        )}
                    </p>
                </div>
                <div className="flex items-center gap-2 self-start sm:self-auto">
                    <Button
                        variant="outline"
                        size="sm"
                        onClick={async () => {
                            const url = `${window.location.origin}/preferences/${project.id}/${user.id}`
                            await navigator.clipboard.writeText(url)
                            toast.success(t("copied_to_clipboard", "Copied to clipboard"))
                        }}
                        className="shrink-0"
                    >
                        <Link2 className="mr-2 h-4 w-4" />
                        {t("copy_preferences_link", "Copy preferences link")}
                    </Button>
                    {subscriptions.length > 0 && (
                        <Button
                            variant="outline"
                            size="sm"
                            onClick={handleUnsubscribeAll}
                            className="shrink-0"
                        >
                            <BellOff className="mr-2 h-4 w-4" />
                            {t("unsubscribe_all")}
                        </Button>
                    )}
                </div>
            </div>

            {/* Subscriptions Table */}
            <div className="border rounded-lg">
                <Table>
                    <TableHeader>
                        <TableRow>
                            <TableHead>{t("name")}</TableHead>
                            <TableHead className="hidden sm:table-cell">{t("channel")}</TableHead>
                            <TableHead className="w-28 text-right">{t("subscribed")}</TableHead>
                        </TableRow>
                    </TableHeader>
                    <TableBody>
                        {search.isPending ? (
                            Array.from({ length: 3 }).map((_, i) => (
                                <TableRow key={i}>
                                    <TableCell>
                                        <Skeleton className="h-4 w-32" />
                                    </TableCell>
                                    <TableCell className="hidden sm:table-cell">
                                        <Skeleton className="h-4 w-20" />
                                    </TableCell>
                                    <TableCell className="text-right">
                                        <Skeleton className="h-5 w-9 ml-auto" />
                                    </TableCell>
                                </TableRow>
                            ))
                        ) : search.isError ? (
                            <LoadErrorRow
                                colSpan={3}
                                message={t(
                                    "user_subscriptions_load_failed",
                                    "Couldn't load subscriptions.",
                                )}
                                onRetry={() => search.refetch()}
                            />
                        ) : subscriptions.length === 0 ? (
                            <TableRow>
                                <TableCell colSpan={3} className="h-48">
                                    <div className="flex flex-col items-center justify-center">
                                        <div className="flex h-12 w-12 items-center justify-center rounded-full bg-muted mb-4">
                                            <Bell className="h-6 w-6 text-muted-foreground" />
                                        </div>
                                        <p className="font-medium mb-1">
                                            {t("no_subscriptions_yet", "No subscriptions")}
                                        </p>
                                        <p className="text-sm text-muted-foreground max-w-xs text-center">
                                            {t(
                                                "no_subscriptions_description",
                                                "Subscriptions will appear here when configured",
                                            )}
                                        </p>
                                    </div>
                                </TableCell>
                            </TableRow>
                        ) : (
                            subscriptions.map((sub) => (
                                <TableRow key={sub.subscription_id}>
                                    <TableCell className="font-medium">{sub.name}</TableCell>
                                    <TableCell className="hidden sm:table-cell">
                                        <span className="text-sm text-muted-foreground bg-muted px-2 py-0.5 rounded">
                                            {snakeToTitle(sub.channel)}
                                        </span>
                                    </TableCell>
                                    <TableCell className="text-right">
                                        <Switch
                                            checked={sub.state === "subscribed"}
                                            onCheckedChange={(checked) =>
                                                handleToggle(
                                                    sub.subscription_id,
                                                    checked ? "subscribed" : "unsubscribed",
                                                )
                                            }
                                        />
                                    </TableCell>
                                </TableRow>
                            ))
                        )}
                    </TableBody>
                </Table>
            </div>

            {/* Confirmation Dialog */}
            <Dialog
                open={confirmAction !== null}
                onOpenChange={(open) => !open && setConfirmAction(null)}
            >
                <DialogContent>
                    <DialogHeader>
                        <DialogTitle>
                            {confirmAction?.type === "unsubscribe_all"
                                ? t("unsubscribe_all")
                                : t("change_subscription_status", "Change subscription status")}
                        </DialogTitle>
                        <DialogDescription>
                            {confirmAction?.type === "unsubscribe_all"
                                ? t("users_unsubscribe_all")
                                : t("users_change_subscription_status")}
                        </DialogDescription>
                    </DialogHeader>
                    <DialogFooter>
                        <Button
                            variant="outline"
                            onClick={() => setConfirmAction(null)}
                            disabled={isSaving}
                        >
                            {t("cancel")}
                        </Button>
                        <Button onClick={executeAction} disabled={isSaving}>
                            {t("confirm")}
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>
        </div>
    )
}
