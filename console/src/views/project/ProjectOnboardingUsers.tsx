import { useNavigate, useParams } from "react-router"
import { useTranslation } from "react-i18next"
import { AxiosError } from "axios"
import { toast } from "sonner"
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
import type { User } from "../../types"
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
            identifier: [
                { source: "admin", external_id: admin.id },
            ] as unknown as User["identifier"],
            data: {
                full_name: fullName,
                admin: true,
            },
            email: admin.email,
            timezone: project.timezone,
        })
    }

    function getErrorDetail(err: unknown) {
        if (err instanceof AxiosError && typeof err.response?.data?.detail === "string") {
            return err.response.data.detail
        }

        return null
    }

    const next = async () => {
        setNextLoading(true)
        try {
            if (file) {
                try {
                    await api.users.addImport(projectId, file)
                } catch (err) {
                    toast.error(
                        getErrorDetail(err) ||
                            t("onboarding_users_import_failed", "Failed to import users from CSV."),
                    )
                    return
                }
            }

            try {
                await createInitialUser()
            } catch (err) {
                toast.error(
                    getErrorDetail(err) ||
                        t(
                            "onboarding_users_default_user_create_failed",
                            "Failed to create the default admin user.",
                        ),
                )
                return
            }

            await navigate(`/projects/${projectId}/onboarding/getting-started`)
        } finally {
            setNextLoading(false)
        }
    }

    async function skip() {
        setSkipLoading(true)
        try {
            try {
                await createInitialUser()
            } catch (err) {
                toast.error(
                    getErrorDetail(err) ||
                        t(
                            "onboarding_users_default_user_create_failed",
                            "Failed to create the default admin user.",
                        ),
                )
                return
            }

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
