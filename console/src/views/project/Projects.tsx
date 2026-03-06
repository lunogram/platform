import { useContext, useEffect, useMemo, useState } from "react"
import { useNavigate } from "react-router"
import { oapiClient } from "@/oapi/client"
import { useResolver } from "../../hooks"
import type { Project } from "../../types"
import { Button } from "@/components/ui/button"
import PageContent from "@/components/page-content"
import { PreferencesContext } from "@/contexts/PreferencesContext"
import Tile, { TileGrid } from "@/components/tile"
import { formatDate, getRecentProjects } from "../../utils"
import { PlusIcon } from "../../components/icons"
import Modal from "@/components/modal"
import ProjectForm from "./ProjectForm"
import { useTranslation } from "react-i18next"

export function Projects() {
    const navigate = useNavigate()
    const { t } = useTranslation()
    const [preferences] = useContext(PreferencesContext)
    const [res] = useResolver(() => oapiClient.GET('/api/admin/projects', {
            params: {
                query: {
                    limit: 50,
                    offset: 0,
                },
            },
         })
    )
    const projects = res?.data?.results || [];
    const recents = useMemo(() => {
        const recents = getRecentProjects()
        if (!projects?.length || !recents.length) return []
        return recents.reduce<
            Array<{
                project: Project
                when: number
            }>
        >((a, { id, when }) => {
            const project = projects.find((p) => p.id === id)
            if (project) {
                a.push({
                    when,
                    project,
                });
            }
            return a;
        }, []);
    }, [projects]);
    const [open, setOpen] = useState(false);

    useEffect(() => {
        if (res && projects && !projects.length) {
            navigate("/onboarding/project")?.catch((e) => {
                console.error("Failed to navigate to onboarding:", e)
            })
        }
    }, [res, projects, navigate]);

    if (!res) return null;

    return (
        <PageContent
            title={t("projects")}
            desc={t("projects_description")}
            actions={
                <Button variant="default" onClick={() => setOpen(true)}>
                    <PlusIcon />
                    {t("create_project")}
                </Button>
            }
        >
            {!!recents?.length && (
                <>
                    <h3 className="legacy-typography">{t("recently_viewed")}</h3>
                    <TileGrid>
                        {recents.map(({ project, when }) => (
                            <Tile
                                key={project.id}
                                onClick={async () => {
                                    await navigate("/projects/" + project.id)
                                }}
                                title={project.name || "Untitled Project"}
                                iconUrl={""}
                            >
                                {formatDate(preferences, when)}
                            </Tile>
                        ))}
                    </TileGrid>
                </>
            )}
            <h3 className="legacy-typography">{t("projects_all")}</h3>
            <TileGrid>
                {projects?.map((project) => (
                    <Tile
                        key={project.id}
                        onClick={async () => {
                            await navigate("/projects/" + project.id)
                        }}
                        title={project.name}
                        iconUrl={""}
                    >
                        {formatDate(preferences, project.created_at)}
                    </Tile>
                ))}
            </TileGrid>
            <Modal open={open} onClose={setOpen} title={t("create_project")} size="regular">
                <ProjectForm
                    onSave={async (project) => {
                        setOpen(false)
                        await navigate("/projects/" + project.id)
                    }}
                />
            </Modal>
        </PageContent>
    )
}
