import { Link, useNavigate, useParams } from "react-router"
import { useTranslation } from "react-i18next"
import { Button } from "@/components/ui/button"
import {
    Card,
    CardContent,
    CardDescription,
    CardFooter,
    CardHeader,
    CardTitle,
} from "@/components/ui/card"
import type { UUID } from "@/types/common"
import { NIL } from "uuid"
import { Megaphone, Route, Loader2 } from "lucide-react"
import { useCreateOnboardingJourney } from "./useCreateOnboardingJourney"

export default function ProjectOnboardingGettingStarted() {
    const navigate = useNavigate()
    const { t } = useTranslation()
    const { projectId = NIL as UUID } = useParams<{ projectId: UUID }>()
    const createJourney = useCreateOnboardingJourney(projectId, { reuseExisting: true })
    const isJourneyLoading = createJourney.isPending
    const createOnboardingJourney = () => createJourney.mutate()

    async function createCampaign() {
        await navigate(`/projects/${projectId}/campaigns/new`)
    }

    return (
        <Card className="w-full max-w-[600px]">
            <CardHeader>
                <CardTitle className="text-lg">{t("getting-started")}</CardTitle>
                <CardDescription>
                    {t(
                        "onboarding_getting_started_description",
                        "Choose how you want to start reaching your users.",
                    )}
                </CardDescription>
            </CardHeader>
            <CardContent>
                <div className="grid grid-cols-2 gap-3">
                    <button
                        type="button"
                        onClick={createOnboardingJourney}
                        disabled={isJourneyLoading}
                        className="flex flex-col items-center gap-3 rounded-lg border border-border p-6 text-center transition-colors hover:bg-accent disabled:opacity-70 cursor-pointer disabled:cursor-not-allowed"
                    >
                        {isJourneyLoading ? (
                            <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
                        ) : (
                            <Route className="h-6 w-6 text-muted-foreground" />
                        )}
                        <span className="text-sm font-medium">
                            {t("onboarding_project-getting-started_journey")}
                        </span>
                    </button>
                    <button
                        type="button"
                        onClick={createCampaign}
                        className="flex flex-col items-center gap-3 rounded-lg border border-border p-6 text-center transition-colors hover:bg-accent cursor-pointer"
                    >
                        <Megaphone className="h-6 w-6 text-muted-foreground" />
                        <span className="text-sm font-medium">
                            {t("onboarding_project-getting-started_campaign")}
                        </span>
                    </button>
                </div>
            </CardContent>
            <CardFooter>
                <Link to={`/projects/${projectId}/getting-started`}>
                    <Button variant="outline">{t("skip")}</Button>
                </Link>
            </CardFooter>
        </Card>
    )
}
