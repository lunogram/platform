import { useContext, useEffect, useState } from "react"

import {
    BookIcon,
    CampaignsIcon,
    CheckCircleIcon,
    JourneysIcon,
    ListsIcon,
    UsersIcon,
} from "@/components/icons"

import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { ProjectContext } from "@/contexts"
import { useNavigate, useParams } from "react-router"
import type { UUID } from "@/types/common"
import type { Project } from "@/types"
import { NIL } from "uuid"
import { oapiClient } from "@/oapi/client"
import type { components } from "@/oapi/management.generated"
import { cn } from "@/utils"
import { t } from "i18next"
import { Puzzle } from "lucide-react"

export default function ProjectGettingStarted() {
    const navigate = useNavigate()
    const { projectId = NIL as UUID } = useParams<{ projectId: UUID }>()
    const [project, setProject] = useContext(ProjectContext)
    const [isJourneyLoading, setIsJourneyLoading] = useState(false)

    useEffect(() => {
        const loadProject = async () => {
            const { data } = await oapiClient.GET("/api/admin/projects/{projectID}", {
                params: { path: { projectID: projectId } },
            })
            if (data) setProject(data as Project)
        }
        loadProject().catch(console.error)
    }, [setProject, projectId])

    const hasCampaigns = (project.campaigns_count ?? 0) > 0
    const hasJourneys = (project.journeys_count ?? 0) > 0
    const hasUsers = (project.users_count ?? 0) > 0
    const hasLists = (project.lists_count ?? 0) > 0
    const hasIntegrations = (project.integrations_count ?? 0) > 0

    async function createOnboardingJourney() {
        setIsJourneyLoading(true)
        try {
            const { data: journey } = await oapiClient.POST(
                "/api/admin/projects/{projectID}/journeys",
                {
                    params: { path: { projectID: projectId } },
                    body: {
                        name: "Onboarding",
                        description: "Getting started with your first journey",
                        template_id: "onboarding",
                        status: "draft",
                    } as components["schemas"]["CreateJourney"],
                },
            )
            if (journey) await navigate(`/projects/${projectId}/journeys/${journey.id}`)
        } finally {
            setIsJourneyLoading(false)
        }
    }

    const checklistItems = [
        {
            icon: <Puzzle className="h-4 w-4" />,
            completed: hasIntegrations,
            title: t("project.getting_started.checklist.integration.title"),
            description: t("project.getting_started.checklist.integration.description"),
            action: (
                <Button variant="secondary" onClick={() => navigate("../integrations")}>
                    {t("project.getting_started.checklist.integration.action")}
                </Button>
            ),
        },
        {
            icon: <CampaignsIcon />,
            completed: hasCampaigns,
            title: t("project.getting_started.checklist.campaign.title"),
            description: t("project.getting_started.checklist.campaign.description"),
            action: (
                <Button variant="secondary" onClick={() => navigate("../campaigns/new")}>
                    {t("project.getting_started.checklist.campaign.action")}
                </Button>
            ),
        },
        {
            icon: <JourneysIcon />,
            completed: hasJourneys,
            title: t("project.getting_started.checklist.journey.title"),
            description: t("project.getting_started.checklist.journey.description"),
            action: (
                <Button
                    variant="secondary"
                    onClick={createOnboardingJourney}
                    isLoading={isJourneyLoading}
                >
                    {t("project.getting_started.checklist.journey.action")}
                </Button>
            ),
        },
        {
            icon: <UsersIcon />,
            completed: hasUsers,
            title: t("project.getting_started.checklist.users.title"),
            description: t("project.getting_started.checklist.users.description"),
            action: (
                <Button variant="secondary" onClick={() => navigate("../users")}>
                    {t("project.getting_started.checklist.users.action")}
                </Button>
            ),
        },
        {
            icon: <ListsIcon />,
            completed: hasLists,
            title: t("project.getting_started.checklist.lists.title"),
            description: t("project.getting_started.checklist.lists.description"),
            action: (
                <Button variant="secondary" onClick={() => navigate("../lists")}>
                    {t("project.getting_started.checklist.lists.action")}
                </Button>
            ),
        },
    ]

    return (
        <div className="flex flex-col gap-4 sm:gap-6 p-4 sm:p-6 max-w-3xl mx-auto min-h-screen justify-center">
            {/* Checklist */}
            <Card className="border rounded-lg">
                <CardHeader className="border-b">
                    <CardTitle className="text-lg font-semibold">
                        {t("project.getting_started.title")}
                    </CardTitle>
                </CardHeader>
                <CardContent className="pt-2">
                    <ul className="divide-y divide-border">
                        {checklistItems.map((item, i) => (
                            <li
                                key={i}
                                className="flex flex-col sm:flex-row sm:items-center gap-3 sm:gap-4 py-4 first:pt-2 last:pb-2"
                            >
                                <div className="flex items-center gap-3 sm:gap-4 flex-1 min-w-0">
                                    <div
                                        className={cn(
                                            "w-6 h-6 flex items-center justify-center shrink-0 text-muted-foreground [&>svg]:w-full [&>svg]:h-full",
                                            item.completed && "text-green-600",
                                        )}
                                    >
                                        {item.completed ? <CheckCircleIcon /> : item.icon}
                                    </div>
                                    <div className="flex flex-col flex-1 gap-1 min-w-0">
                                        <strong className="text-sm font-medium">
                                            {item.title}
                                        </strong>
                                        <small className="text-xs text-muted-foreground">
                                            {item.description}
                                        </small>
                                    </div>
                                </div>
                                {!item.completed && (
                                    <div className="sm:shrink-0 pl-9 sm:pl-0">{item.action}</div>
                                )}
                            </li>
                        ))}
                    </ul>
                </CardContent>
            </Card>

            {/* Resources */}
            <Card className="border rounded-lg">
                <CardContent className="flex flex-col sm:flex-row gap-8 p-4 sm:p-6">
                    <div className="flex-1 sm:border-r sm:border-border last:border-none sm:pr-6">
                        <div className="w-6 h-6 mb-2 text-muted-foreground">
                            <BookIcon />
                        </div>
                        <h4 className="text-sm font-semibold mb-1">
                            {t("project.getting_started.documentation.title")}
                        </h4>
                        <p className="text-sm text-muted-foreground">
                            {t("project.getting_started.documentation.description")}
                        </p>
                    </div>
                </CardContent>
            </Card>
        </div>
    )
}
