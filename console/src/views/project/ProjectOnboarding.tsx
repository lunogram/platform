import { useContext } from "react"
import { Outlet, useLocation, useParams } from "react-router"
import { useQuery } from "@tanstack/react-query"
import { ProjectContext } from "../../contexts"
import { cn } from "@/utils"
import { isEnterprise } from "@/config/enterprise"
import { providersQuery } from "../settings/queries"

export default function ProjectOnboarding() {
    const location = useLocation()
    const { projectId = "" } = useParams()
    const [project] = useContext(ProjectContext)
    const base = `/projects/${projectId}/onboarding`

    const hasIntegrations = (project?.integrations_count ?? 0) > 0

    // Whether a courier provider exists decides if the domain step is shown on
    // every onboarding page. Read from the shared providers query, so creating
    // one in the first step adds the step right away.
    const { data: hasProvider } = useQuery({
        ...providersQuery(projectId),
        enabled: isEnterprise,
        select: (providers) => providers.some((p) => p.module === "courier"),
    })
    const showDomainStep = hasProvider === true

    const steps = hasIntegrations
        ? showDomainStep
            ? (["domain", "users", "getting-started"] as const)
            : (["users", "getting-started"] as const)
        : showDomainStep
          ? (["", "domain", "users", "getting-started"] as const)
          : (["", "users", "getting-started"] as const)

    const currentStep = steps.findIndex((s) =>
        s === ""
            ? location.pathname === base || location.pathname === base + "/"
            : location.pathname.endsWith("/" + s),
    )

    return (
        <div className="flex min-h-screen flex-col items-center justify-center gap-6 bg-muted/40 p-4 sm:p-10">
            {/* Step indicator */}
            <div className="flex items-center gap-1.5">
                {steps.map((_, i) => (
                    <div key={i} className="flex items-center gap-1.5">
                        {i > 0 && (
                            <div
                                className={cn(
                                    "h-px w-6",
                                    i <= currentStep ? "bg-primary" : "bg-border",
                                )}
                            />
                        )}
                        <div
                            className={cn(
                                "h-2 w-2 rounded-full",
                                i === currentStep
                                    ? "bg-primary"
                                    : i < currentStep
                                      ? "bg-primary/40"
                                      : "bg-border",
                            )}
                        />
                    </div>
                ))}
            </div>

            {/* Step content */}
            <Outlet />
        </div>
    )
}
