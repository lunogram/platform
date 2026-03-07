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
import api from "../../api"
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
        const admin = await api.admins.whoami()
        if (!admin) return

        let fullName
        if (admin.first_name || admin.last_name) {
            fullName = admin.last_name ? admin.first_name + " " + admin.last_name : admin.first_name
        }

        await api.users.create(projectId, {
            anonymous_id: crypto.randomUUID(),
            data: {
                full_name: fullName,
                admin: true,
            },
            email: admin.email,
            timezone: project.timezone,
        })
    }

    const next = async () => {
        setNextLoading(true)
        try {
            await createInitialUser()

            if (file) {
                await api.users.addImport(projectId, file)
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
