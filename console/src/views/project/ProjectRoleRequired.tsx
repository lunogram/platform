import type { PropsWithChildren } from "react"
import { useContext } from "react"
import { ProjectContext } from "../../contexts"
import type { ProjectRole } from "../../types"
import { projectRoles } from "../../types"
import { AccessDenied } from "../ErrorPage"

type ProjectRoleRequiredProps = PropsWithChildren<{
    minRole: ProjectRole
}>

export function ProjectRoleRequired({ children, minRole }: ProjectRoleRequiredProps) {
    const [project] = useContext(ProjectContext)

    if (!project.role || projectRoles.indexOf(minRole) > projectRoles.indexOf(project.role)) {
        return <AccessDenied />
    }

    return <>{children}</>
}
