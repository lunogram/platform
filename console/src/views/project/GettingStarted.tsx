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
import { NIL } from "uuid"
import api from "@/api"
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
            const projectState = await api.projects.get(projectId)
            setProject(projectState)
        }
        loadProject().catch(console.error)
    }, [setProject, projectId])

    // Probably in App.tsx or useEffect somewhere
    useEffect(() => {
        // Check if browser even supports this cursed API
        if (!("serviceWorker" in navigator)) {
            console.log("Browser said no to service workers, RIP")
            return
        }

        if (!("Notification" in window)) {
            console.log("This browser is from 2009 apparently")
            return
        }

        console.log("Permission status:", Notification.permission)

        Notification.requestPermission().then((permission) => {
            if (permission === "granted") {
                // Now you can test SW notifications
                console.log("User said yes, we can annoy them now")

                // Register the service worker
                navigator.serviceWorker
                    .register("/sw.js", { scope: "/" }) // path relative to your domain root
                    .then((registration) => {
                        console.log("SW registered, somehow:", registration)
                    })
                    .catch((error) => {
                        console.error("SW registration ate shit:", error)
                    })
                    .finally(() => {
                        navigator.serviceWorker.getRegistration().then((reg) => {
                            console.log("Current SW:", reg)
                            if (reg && reg.active) {
                                console.log("SW is active, script URL:", reg.active.scriptURL)
                            }
                        })

                        navigator.serviceWorker.ready.then((reg) => {
                            reg.showNotification("Manual Test", {
                                body: "If this shows, notifications work",
                            })
                        })

                        navigator.serviceWorker.controller?.postMessage({
                            type: "test",
                            payload: "hello from main thread",
                        })
                    })
            } else if (permission === "denied") {
                console.log("User said fuck off")
                // They have to manually unblock in browser settings now
            } else {
                console.log("User dismissed, coward")
            }
        })
    }, []) // Empty deps = run once on mount

    const hasCampaigns = (project.campaigns_count ?? 0) > 0
    const hasJourneys = (project.journeys_count ?? 0) > 0
    const hasUsers = (project.users_count ?? 0) > 0
    const hasLists = (project.lists_count ?? 0) > 0
    const hasIntegrations = (project.integrations_count ?? 0) > 0

    async function createOnboardingJourney() {
        setIsJourneyLoading(true)
        try {
            const journey = await api.journeys.create(projectId, {
                name: "Onboarding",
                description: "Getting started with your first journey",
                template_id: "onboarding",
                status: "draft",
            })
            await navigate(`/projects/${projectId}/journeys/${journey.id}`)
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
