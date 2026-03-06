import { useCallback, useContext, useState } from "react"
import { useTranslation } from "react-i18next"
import { Bell, BellOff } from "lucide-react"
import { ProjectContext, UserContext } from "../../contexts"
import { useResolver } from "../../hooks"
import { snakeToTitle } from "../../utils"
import { oapiClient } from "@/oapi/client"
import type { SubscriptionParams, SubscriptionState } from "../../types"
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

    const [search, , reload] = useResolver(
        useCallback(async () => {
            const res = await oapiClient.GET('/api/admin/projects/{projectID}/users/{userID}/subscriptions', {
                params: {
                    path: {
                        projectID: project.id,
                        userID: user.id,
                    },
                    query: { limit: 100 },
                },
            })
            return res.data
        }, [project.id, user.id]),
    )

    const subscriptions = search?.results

    const handleToggle = async (subscriptionId: UUID, newState: SubscriptionState) => {
        setConfirmAction({ type: "toggle", subscriptionId, newState })
    }

    const handleUnsubscribeAll = () => {
        setConfirmAction({ type: "unsubscribe_all" })
    }

    const executeAction = async () => {
        if (!confirmAction) return

        if (confirmAction.type === "toggle" && confirmAction.subscriptionId) {
            await oapiClient.PATCH('/api/admin/projects/{projectID}/users/{userID}/subscriptions', {
                params: {
                    path: {
                        projectID: project.id,
                        userID: user.id,
                    },
                },
                body: [
                    {
                        subscription_id: confirmAction.subscriptionId,
                        state: confirmAction.newState!,
                    },
                ],
            })
        } else if (confirmAction.type === "unsubscribe_all") {
            const params: SubscriptionParams[] =
                subscriptions?.map((item) => ({
                    subscription_id: item.subscription_id,
                    state: "unsubscribed" as SubscriptionState,
                })) ?? []
            await oapiClient.PATCH('/api/admin/projects/{projectID}/users/{userID}/subscriptions', {
                params: {
                    path: {
                        projectID: project.id,
                        userID: user.id,
                    },
                },
                body: params,
            })
        }

        await reload()
        setConfirmAction(null)
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
                {subscriptions && subscriptions.length > 0 && (
                    <Button
                        variant="outline"
                        size="sm"
                        onClick={handleUnsubscribeAll}
                        className="self-start sm:self-auto shrink-0"
                    >
                        <BellOff className="mr-2 h-4 w-4" />
                        {t("unsubscribe_all")}
                    </Button>
                )}
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
                        {subscriptions === undefined ? (
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
                        <Button variant="outline" onClick={() => setConfirmAction(null)}>
                            {t("cancel")}
                        </Button>
                        <Button onClick={executeAction}>{t("confirm")}</Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>
        </div>
    )
}
