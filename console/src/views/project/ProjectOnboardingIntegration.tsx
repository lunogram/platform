import { useCallback, useContext, useState } from "react"
import { useNavigate, useParams } from "react-router"
import { useTranslation } from "react-i18next"
import { ChevronLeft } from "lucide-react"
import { NIL } from "uuid"
import { oapiClient } from "@/oapi/client"
import { ProjectContext } from "../../contexts"
import { useResolver } from "../../hooks"
import { snakeToTitle, hasCourierProvider } from "../../utils"
import { IntegrationForm } from "../settings/IntegrationModal"
import { Button } from "@/components/ui/button"
import {
    Card,
    CardContent,
    CardDescription,
    CardFooter,
    CardHeader,
    CardTitle,
} from "@/components/ui/card"
import { isEnterprise } from "@/config/enterprise"
import type { UUID } from "@/types/common"
import type { Project } from "../../types"
import type { ProviderMeta } from "@/oapi/client"

export default function ProjectOnboardingIntegration() {
    const navigate = useNavigate()
    const { t } = useTranslation()
    const { projectId = NIL as UUID } = useParams<{ projectId: UUID }>()
    const [project, setProject] = useContext(ProjectContext)
    const [meta, setMeta] = useState<ProviderMeta | undefined>()

    const [options] = useResolver(
        useCallback(async () => {
            const { data } = await oapiClient.GET(
                "/api/admin/projects/{projectID}/providers/meta",
                {
                    params: { path: { projectID: projectId } },
                },
            )
            return data
        }, [projectId]),
    )

    const [hasProvider] = useResolver(useCallback(() => hasCourierProvider(projectId), [projectId]))
    const nextStep = isEnterprise && hasProvider === true ? "domain" : "users"

    async function handleSkip() {
        await navigate(`/projects/${projectId}/onboarding/${nextStep}`)
    }

    return (
        <Card className="w-full max-w-[600px]">
            <CardHeader>
                <CardTitle className="text-lg">{t("onboarding_integration_title")}</CardTitle>
                <CardDescription>{t("onboarding_integration_description")}</CardDescription>
            </CardHeader>
            <CardContent>
                {!meta ? (
                    <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
                        {options?.map((option) => (
                            <button
                                key={`${option.group}${option.type}`}
                                type="button"
                                className="flex flex-col items-center gap-2 rounded-lg border p-4 text-center transition-colors hover:bg-accent hover:text-accent-foreground"
                                onClick={() => setMeta(option)}
                            >
                                {option.icon && (
                                    <img
                                        src={option.icon}
                                        alt={option.name}
                                        className="h-10 w-10 rounded-md object-contain"
                                    />
                                )}
                                <div>
                                    <p className="text-sm font-medium">{option.name}</p>
                                    <p className="text-xs text-muted-foreground">
                                        {snakeToTitle(option.group)}
                                    </p>
                                </div>
                            </button>
                        ))}
                    </div>
                ) : (
                    <>
                        <Button
                            variant="ghost"
                            size="sm"
                            className="mb-4 w-fit"
                            onClick={() => setMeta(undefined)}
                        >
                            <ChevronLeft className="mr-1 h-4 w-4" />
                            {t("integrations")}
                        </Button>
                        <IntegrationForm
                            project={project}
                            meta={meta}
                            onChange={async () => {
                                const { data: updatedProject } = await oapiClient.GET(
                                    "/api/admin/projects/{projectID}",
                                    { params: { path: { projectID: projectId } } },
                                )
                                if (updatedProject) setProject(updatedProject as Project)
                                const step =
                                    isEnterprise && (await hasCourierProvider(projectId))
                                        ? "domain"
                                        : "users"
                                await navigate(`/projects/${projectId}/onboarding/${step}`)
                            }}
                        />
                    </>
                )}
            </CardContent>
            {!meta && (
                <CardFooter>
                    <Button variant="outline" onClick={handleSkip}>
                        {t("skip")}
                    </Button>
                </CardFooter>
            )}
        </Card>
    )
}
