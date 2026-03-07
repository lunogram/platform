import { useContext, useState } from "react"
import { ProjectContext } from "../../contexts"
import { toast } from "sonner"
import ProjectForm from "../project/ProjectForm"
import { useTranslation } from "react-i18next"
import api from "../../api"
import { AlertTriangle } from "lucide-react"

import { Button } from "@/components/ui/button"
import { Separator } from "@/components/ui/separator"
import { Input } from "@/components/ui/input"
import { Card, CardContent, CardFooter } from "@/components/ui/card"
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog"

export default function ProjectSettings() {
    const { t } = useTranslation()
    const [project, setProject] = useContext(ProjectContext)
    const [isDeleteOpen, setIsDeleteOpen] = useState(false)
    const [isDeleting, setIsDeleting] = useState(false)
    const [confirmName, setConfirmName] = useState("")

    const deleteProject = async () => {
        setIsDeleting(true)
        try {
            await api.projects.delete(project.id)
            window.location.href = "/"
        } catch {
            toast.error(t("delete_project_error"))
            setIsDeleting(false)
        }
    }

    return (
        <>
            <ProjectForm
                project={project}
                onSave={(project) => {
                    setProject(project)
                    toast.success(t("project_settings_saved"))
                }}
            />

            <Separator className="my-8" />

            {/* Danger Zone */}
            <Card className="border-destructive/20 bg-destructive/[0.02] shadow-none">
                <CardContent className="pt-6">
                    <div className="flex items-center gap-3">
                        <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-destructive/10">
                            <AlertTriangle className="h-4 w-4 text-destructive" />
                        </div>
                        <div>
                            <h3 className="font-semibold leading-none tracking-tight">
                                {t("danger_zone")}
                            </h3>
                            <p className="text-sm text-muted-foreground">
                                {t("danger_zone_description")}
                            </p>
                        </div>
                    </div>
                </CardContent>
                <CardFooter className="border-t border-destructive/10 bg-destructive/[0.03] rounded-b-xl justify-between gap-4 !py-4">
                    <p className="text-sm text-muted-foreground">
                        {t("delete_project_description")}
                    </p>
                    <Button variant="destructive" size="sm" onClick={() => setIsDeleteOpen(true)}>
                        {t("delete_project")}
                    </Button>
                </CardFooter>
            </Card>

            {/* Delete Confirmation Dialog */}
            <Dialog
                open={isDeleteOpen}
                onOpenChange={(open) => {
                    setIsDeleteOpen(open)
                    if (!open) setConfirmName("")
                }}
            >
                <DialogContent>
                    <DialogHeader>
                        <DialogTitle>{t("delete_project")}</DialogTitle>
                        <DialogDescription>{t("delete_project_warning")}</DialogDescription>
                    </DialogHeader>
                    <div className="space-y-4 py-4">
                        <div className="flex items-center gap-3 p-3 rounded-lg bg-muted">
                            <div className="flex h-10 w-10 items-center justify-center rounded-lg shrink-0 bg-destructive/10">
                                <AlertTriangle className="h-5 w-5 text-destructive" />
                            </div>
                            <div>
                                <p className="font-medium">{project.name}</p>
                                <p className="text-sm text-muted-foreground">
                                    {project.description || project.id}
                                </p>
                            </div>
                        </div>
                        <div className="space-y-2">
                            <p className="text-sm text-muted-foreground">
                                {t("delete_project_confirm_prompt", { name: project.name })}
                            </p>
                            <Input
                                value={confirmName}
                                onChange={(e) => setConfirmName(e.target.value)}
                                placeholder={project.name}
                            />
                        </div>
                    </div>
                    <DialogFooter>
                        <Button
                            variant="outline"
                            onClick={() => setIsDeleteOpen(false)}
                            disabled={isDeleting}
                        >
                            {t("cancel")}
                        </Button>
                        <Button
                            variant="destructive"
                            onClick={deleteProject}
                            disabled={isDeleting || confirmName !== project.name}
                        >
                            {isDeleting ? t("deleting") : t("delete_project")}
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>
        </>
    )
}
