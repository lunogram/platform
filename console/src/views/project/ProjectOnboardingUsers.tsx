import { useNavigate, useParams } from "react-router"
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
import { UserImportForm } from "@/components/ui/user-import-dialog"
import { oapiClient } from "@/oapi/client"
import type { UUID } from "@/types/common"
import { useContext, useState } from "react"
import { NIL } from "uuid"
import { ProjectContext } from "@/contexts"

export default function ProjectOnboardingUsers() {
    const navigate = useNavigate()
    const { t } = useTranslation()
    const { projectId = NIL as UUID } = useParams<{ projectId: UUID }>()
    const [project] = useContext(ProjectContext)
    const [file, setFile] = useState<File | null>(null)
    const [nextLoading, setNextLoading] = useState(false)
    const [skipLoading, setSkipLoading] = useState(false)

    async function createInitialUser() {
        const res = await oapiClient.GET("/api/admin/organizations/whoami")
        if (!res.data) return

        let fullName
        if (res.data.first_name || res.data.last_name) {
            fullName = res.data.last_name ? res.data.first_name + " " + res.data.last_name : res.data.first_name
        }

        await oapiClient.POST("/api/admin/projects/{projectID}/users", {
            params: {
                path: {
                    projectID: projectId,
                },
            },
            body: {
                anonymous_id: crypto.randomUUID(),
                data: {
                    full_name: fullName,
                    admin: true,
                },
                email: res.data.email,
                timezone: project.timezone,
            },
        })
    }

    const next = async () => {
        setNextLoading(true)
        try {
            await createInitialUser()

            if (file) {
                const formData = new FormData()
                formData.append("file", file)
                await fetch(`/api/admin/projects/${projectId}/users/import`, {
                    method: "POST",
                    credentials: "include",
                    body: formData,
                })
            }

            await navigate(`/projects/${projectId}/onboarding/getting-started`)
        } finally {
            setNextLoading(false)
        }
    }

    async function skip() {
        setSkipLoading(true)
        try {
            await createInitialUser()
            await navigate(`/projects/${projectId}/onboarding/getting-started`)
        } finally {
            setSkipLoading(false)
        }
    }

    return (
        <Card className="w-full max-w-[600px]">
            <CardHeader>
                <CardTitle className="text-lg">{t("onboarding_users_title")}</CardTitle>
                <CardDescription>{t("onboarding_users_description")}</CardDescription>
            </CardHeader>
            <CardContent>
                <UserImportForm file={file} onFileChange={setFile} />
            </CardContent>
            <CardFooter className="flex gap-2">
                <Button onClick={next} isLoading={nextLoading}>
                    {t("next")}
                </Button>
                <Button onClick={skip} isLoading={skipLoading} variant="outline">
                    {t("skip")}
                </Button>
            </CardFooter>
        </Card>
    )
}
