import type { NavLinkProps } from 'react-router';
import { useNavigate } from 'react-router'
import type { PropsWithChildren, ReactNode } from 'react';
import { useCallback, useContext } from 'react'
import { ProjectContext } from '../../contexts'
import api from '../../api'
import { useResolver } from '../../hooks'
import { SingleSelect } from '../../ui/form/SingleSelect'
import { checkProjectRole, getRecentProjects } from '../../utils'
import type { Project, ProjectRole } from '../../types'
import Sidebar from '../../ui/Sidebar'
import { useTranslation } from 'react-i18next'

interface SidebarProps {
    links?: Array<NavLinkProps & {
        key: string
        icon: ReactNode
        minRole?: ProjectRole
        active?: (project: Project) => boolean
    }>
    prepend?: ReactNode
    append?: ReactNode
}

export default function ProjectSidebar({ children, links }: PropsWithChildren<SidebarProps>) {
    const navigate = useNavigate()
    const { t } = useTranslation()
    const [project] = useContext(ProjectContext)
    const [recents] = useResolver(useCallback(async () => {
        const recentIds = getRecentProjects().filter(p => p.id !== project.id).map(p => p.id)
        const recents: Array<typeof project> = []
        if (recentIds.length) {
            const projects = await api.projects.search({
                limit: recentIds.length,
                id: recentIds,
            }).then(r => r.results ?? [])
            for (const id of recentIds) {
                const recent = projects.find(p => p.id === id)
                if (recent) {
                    recents.push(recent)
                }
            }
        }
        return [
            project,
            ...recents,
            {
                name: t('view_all'),
            },
        ]
    }, [project, t]))

    return (
        project && <Sidebar
            links={
                links?.filter(({ minRole, active }) =>
                    (!minRole || checkProjectRole(minRole, project.role)) && (!active || active(project)),
                ).map(({ ...props }) => props)
            }
            prepend={
                <SingleSelect
                    value={project}
                    onChange={async project => {
                        if (!project.id) {
                            await navigate('/organization/projects')
                        } else {
                            await navigate(`/projects/${project.id}`)
                        }
                    }}
                    options={recents ?? []}
                    getSelectedOptionDisplay={p => (
                        <>
                            <div className="project-switcher-label">{t('project_selector')}</div>
                            <div className="project-switcher-value">{p.name}</div>
                        </>
                    )}
                    hideLabel
                    buttonClassName="project-switcher"
                    variant="minimal"
                />
            }
        >{children}</Sidebar>
    )
}
