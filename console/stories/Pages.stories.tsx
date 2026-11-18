import type { Meta, StoryObj } from "@storybook/react-vite"
import {
    ArchiveRestore,
    ChevronLeft,
    ChevronRight,
    Inbox,
    Mail,
    Megaphone,
    MessageSquareDot,
    MoreHorizontal,
    Search,
    Smartphone,
} from "lucide-react"
import { useContext } from "react"
import { useTranslation } from "react-i18next"

import { CampaignsIcon, PlusIcon } from "@/components/icons"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from "@/components/ui/table"
import { PreferencesContext } from "@/contexts/PreferencesContext"
import { getRandomColor } from "@/lib/colors"
import { formatDate, snakeToTitle } from "@/utils"
import type { CampaignRow } from "./fixtures/campaigns"
import { campaigns } from "./fixtures/campaigns"
import { ConsoleShell } from "./fixtures/shell"

const meta: Meta = {
    title: "Pages/Campaigns",
    parameters: {
        layout: "fullscreen",
        docs: {
            description: {
                component:
                    "Fixture-based visual composition of the Campaigns list (src/views/campaign/Campaigns.tsx). It shows layout and populated, empty, and loading states, not live routes or API behavior.",
            },
        },
    },
}

export default meta
type Story = StoryObj

const channelIcons = { email: Mail, sms: Smartphone, push: MessageSquareDot, inbox: Inbox }

function formatDelivery({ sent, total }: CampaignRow["delivery"]) {
    const ratio = total > 0 ? sent / total : 0
    return `${sent.toLocaleString("en")} (${ratio.toLocaleString("en", { style: "percent" })})`
}

function CampaignsPage({
    rows,
    total = rows.length,
    loading = false,
    hasProvider = true,
}: {
    rows: CampaignRow[]
    total?: number
    loading?: boolean
    hasProvider?: boolean
}) {
    const { t } = useTranslation()
    const [preferences] = useContext(PreferencesContext)

    return (
        <ConsoleShell active="campaigns">
            <div className="flex flex-col gap-4 sm:gap-6 p-4 sm:p-6">
                <div className="flex items-start gap-4">
                    <div className="flex h-14 w-14 items-center justify-center rounded-xl shrink-0 bg-muted [&>svg]:h-7 [&>svg]:w-7 [&>svg]:text-muted-foreground">
                        <CampaignsIcon />
                    </div>
                    <div className="space-y-1">
                        <h1 className="text-2xl font-semibold tracking-tight">
                            {t("campaign.plural")}
                        </h1>
                        <p className="text-sm text-muted-foreground">
                            {t(
                                "campaigns_description",
                                "Create and manage email, SMS, and push notification campaigns to engage your audience.",
                            )}
                        </p>
                    </div>
                </div>

                {!hasProvider && (
                    <Alert>
                        <AlertTitle>{t("setup")}</AlertTitle>
                        <AlertDescription className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
                            <span>{t("setup_integration_description")}</span>
                            <Button className="w-full sm:w-auto">{t("setup_integration")}</Button>
                        </AlertDescription>
                    </Alert>
                )}

                <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3 sm:gap-4">
                    <div className="relative sm:max-w-sm flex-1">
                        <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                        <Input
                            placeholder={t("search_campaigns", "Search campaigns...")}
                            className="pl-9"
                        />
                    </div>
                    <div className="flex items-center gap-2">
                        <Button
                            variant="ghost"
                            size="sm"
                            className="h-8 w-8 p-0"
                            aria-label={t("show_archived", "Show archived")}
                        >
                            <ArchiveRestore className="h-4 w-4" />
                        </Button>
                        <Button size="lg">
                            <PlusIcon /> {t("campaign.create.action")}
                        </Button>
                    </div>
                </div>

                <div className="rounded-lg border bg-card">
                    <Table>
                        <TableHeader>
                            <TableRow>
                                <TableHead>{t("name")}</TableHead>
                                <TableHead className="hidden sm:table-cell">
                                    {t("delivery")}
                                </TableHead>
                                <TableHead className="hidden md:table-cell">
                                    {t("updated_at")}
                                </TableHead>
                                <TableHead className="w-[50px]"></TableHead>
                            </TableRow>
                        </TableHeader>
                        <TableBody>
                            {loading ? (
                                Array.from({ length: 5 }).map((_, i) => (
                                    <TableRow key={i}>
                                        <TableCell>
                                            <div className="flex items-center gap-3">
                                                <Skeleton className="h-8 w-8 rounded-md" />
                                                <div className="space-y-1">
                                                    <Skeleton className="h-4 w-36" />
                                                    <Skeleton className="h-3 w-20" />
                                                </div>
                                            </div>
                                        </TableCell>
                                        <TableCell className="hidden sm:table-cell">
                                            <Skeleton className="h-4 w-24" />
                                        </TableCell>
                                        <TableCell className="hidden md:table-cell">
                                            <Skeleton className="h-4 w-28" />
                                        </TableCell>
                                        <TableCell>
                                            <Skeleton className="h-4 w-8" />
                                        </TableCell>
                                    </TableRow>
                                ))
                            ) : rows.length === 0 ? (
                                <TableRow>
                                    <TableCell colSpan={4} className="h-32 text-center">
                                        <div className="flex flex-col items-center gap-2 text-muted-foreground">
                                            <Megaphone className="h-8 w-8" />
                                            <p>{t("no_campaigns_yet", "No campaigns yet")}</p>
                                        </div>
                                    </TableCell>
                                </TableRow>
                            ) : (
                                rows.map((campaign) => {
                                    const ChannelIcon = channelIcons[campaign.channel]
                                    return (
                                        <TableRow key={campaign.id} className="cursor-pointer">
                                            <TableCell>
                                                <div className="flex items-center gap-3">
                                                    <div
                                                        className="flex h-8 w-8 items-center justify-center rounded-md shrink-0"
                                                        style={{
                                                            backgroundColor: getRandomColor(
                                                                campaign.name,
                                                            ),
                                                        }}
                                                    >
                                                        <ChannelIcon className="h-4 w-4 text-white" />
                                                    </div>
                                                    <div>
                                                        <div className="font-medium">
                                                            {campaign.name}
                                                        </div>
                                                        <div className="text-sm text-muted-foreground">
                                                            {snakeToTitle(campaign.channel)}
                                                        </div>
                                                    </div>
                                                </div>
                                            </TableCell>
                                            <TableCell className="hidden sm:table-cell text-muted-foreground">
                                                {campaign.delivery.sent > 0
                                                    ? formatDelivery(campaign.delivery)
                                                    : "—"}
                                            </TableCell>
                                            <TableCell className="hidden md:table-cell text-muted-foreground">
                                                {formatDate(preferences, campaign.updated_at, "PP")}
                                            </TableCell>
                                            <TableCell>
                                                <Button
                                                    variant="ghost"
                                                    size="sm"
                                                    className="h-8 w-8 p-0"
                                                    aria-label="Actions"
                                                >
                                                    <MoreHorizontal className="h-4 w-4" />
                                                </Button>
                                            </TableCell>
                                        </TableRow>
                                    )
                                })
                            )}
                        </TableBody>
                    </Table>

                    {!loading && rows.length > 0 && (
                        <div className="flex items-center justify-between border-t px-4 py-3">
                            <p className="text-sm text-muted-foreground">
                                {total} {t("campaign.plural").toLowerCase()}
                            </p>
                            {total > rows.length && (
                                <div className="flex items-center gap-2">
                                    <Button variant="outline" size="sm" disabled>
                                        <ChevronLeft className="h-4 w-4 sm:mr-1" />
                                        <span className="hidden sm:inline">{t("previous")}</span>
                                    </Button>
                                    <Button variant="outline" size="sm">
                                        <span className="hidden sm:inline">{t("next")}</span>
                                        <ChevronRight className="h-4 w-4 sm:ml-1" />
                                    </Button>
                                </div>
                            )}
                        </div>
                    )}
                </div>
            </div>
        </ConsoleShell>
    )
}

export const Populated: Story = {
    render: () => <CampaignsPage rows={campaigns} total={32} />,
}

export const Empty: Story = {
    render: () => <CampaignsPage rows={[]} />,
}

export const Loading: Story = {
    render: () => <CampaignsPage rows={[]} loading />,
}

export const SetupRequired: Story = {
    name: "Setup required",
    render: () => <CampaignsPage rows={[]} hasProvider={false} />,
}
