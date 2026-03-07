import PageContent from "@/components/page-content"
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from "@/components/ui/table"
import { Skeleton } from "@/components/ui/skeleton"
import { Badge } from "@/components/ui/badge"
import type { BadgeProps } from "@/components/ui/badge"
import { camelToTitle, formatDate } from "../../utils"
import { useLoaderData } from "react-router"
import type { JourneyEntranceDetail } from "../../types"
import { useContext } from "react"
import { PreferencesContext } from "@/contexts/PreferencesContext"
import * as stepTypes from "./steps"
import clsx from "clsx"
import { useTranslation } from "react-i18next"
import { stepCategoryColors } from "./hooks/JourneyEditor.constants"

// eslint-disable-next-line react-refresh/only-export-components
export const typeVariants: Record<string, BadgeProps["variant"]> = {
    completed: "default",
    error: "destructive",
    campaign: "default",
    delay: "outline",
    pending: "secondary",
}

interface ColumnDef<T> {
    key: string
    title?: string
    cell?: (args: { item: T }) => React.ReactNode
}

export default function EntranceDetails() {
    const { t } = useTranslation()
    const [preferences] = useContext(PreferencesContext)

    const { journey, user, userSteps } = useLoaderData<JourneyEntranceDetail>()

    const entrance = userSteps[0]
    const error = userSteps.find((s) => s.type === "error")
    const displayName = user?.full_name ?? user?.email ?? user?.phone ?? user?.id

    const columns: ColumnDef<(typeof userSteps)[number]>[] = [
        {
            key: "step",
            title: t("step"),
            cell: ({ item }) => {
                const stepType = stepTypes[item.step!.type as keyof typeof stepTypes]

                return (
                    <div className="grid grid-cols-[50px_auto] items-center gap-x-2.5 min-w-[150px]">
                        <div className={clsx("icon-box", stepCategoryColors[stepType.category])}>
                            {stepType?.icon}
                        </div>
                        <div>
                            <div className="font-medium">{item.step!.name || "Untitled"}</div>
                            <div className="text-sm text-muted-foreground mt-0.5">
                                {t(item.step!.type)}
                            </div>
                        </div>
                    </div>
                )
            },
        },
        {
            key: "type",
            title: "Type",
            cell: ({ item }) => (
                <Badge variant={typeVariants[item.type]}>{camelToTitle(item.type)}</Badge>
            ),
        },
        { key: "created_at", title: t("created_at") },
        { key: "delay_until", title: t("delay_until") },
    ]

    const isLoading = !userSteps
    const items = userSteps ?? []

    return (
        <PageContent
            title={`${displayName} - ${journey.name}`}
            desc={
                <>
                    <Badge
                        variant={
                            error ? "destructive" : entrance.ended_at ? "default" : "secondary"
                        }
                    >
                        {error ? "Error" : entrance.ended_at ? "Completed" : "Running"}
                    </Badge>
                    {entrance.ended_at && ` at ${formatDate(preferences, new Date())}`}
                </>
            }
        >
            <div className="rounded-md border">
                <Table>
                    <TableHeader>
                        <TableRow>
                            {columns.map((col) => (
                                <TableHead key={col.key}>{col.title ?? col.key}</TableHead>
                            ))}
                        </TableRow>
                    </TableHeader>
                    <TableBody>
                        {items.length > 0 ? (
                            items.map((item, index) => {
                                const args = { item }
                                const key = (item as any).id ?? index

                                return (
                                    <TableRow key={key}>
                                        {columns.map((col) => {
                                            let value: any = col.cell
                                                ? col.cell(args)
                                                : item[col.key as keyof typeof item]
                                            if (
                                                !col.cell &&
                                                (col.key.endsWith("_at") ||
                                                    col.key.endsWith("_until")) &&
                                                (typeof value === "string" ||
                                                    typeof value === "number")
                                            ) {
                                                value = formatDate(preferences, value, "Pp")
                                            }
                                            return (
                                                <TableCell key={col.key}>
                                                    {value ?? <>&#8211;</>}
                                                </TableCell>
                                            )
                                        })}
                                    </TableRow>
                                )
                            })
                        ) : isLoading ? (
                            Array.from({ length: 3 }, (_, i) => (
                                <TableRow key={i}>
                                    {columns.map((col) => (
                                        <TableCell key={col.key}>
                                            <Skeleton className="h-4 w-full" />
                                        </TableCell>
                                    ))}
                                </TableRow>
                            ))
                        ) : (
                            <TableRow>
                                <TableCell colSpan={columns.length} className="text-center">
                                    No Results
                                </TableCell>
                            </TableRow>
                        )}
                    </TableBody>
                </Table>
            </div>
        </PageContent>
    )
}
