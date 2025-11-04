import { ChevronRight } from "lucide-react"
import { Outlet, useLocation, useNavigate } from "react-router"
import { useContext, useMemo } from "react"
import { ProjectContext } from "@/contexts"

import { Button } from "@/components/ui/button"

import {
    Pagination,
    PaginationContent,
    PaginationItem,
    PaginationLink,
} from "@/components/ui/pagination"

export default function CampaignDetail() {
    const [project] = useContext(ProjectContext)
    const location = useLocation()
    const navigate = useNavigate()

    const campaignId = "00000000-0000-0000-0000-000000000000"

    const steps = useMemo(
        () => [
            { name: "Setup", href: `/projects/${project.id}/campaigns/${campaignId}` },
            { name: "Content", href: `/projects/${project.id}/campaigns/${campaignId}/content` },
            { name: "Review", href: `/projects/${project.id}/campaigns/${campaignId}/review` },
        ],
        [project.id]
    )

    const currentStepIndex = steps.findIndex((s) =>
        location.pathname === s.href
    )

    const nextStep =
        currentStepIndex >= 0 && currentStepIndex < steps.length - 1
            ? steps[currentStepIndex + 1]
            : null

    const isStepActive = (stepHref: string) => {
        return location.pathname === stepHref
    }

    return (
        <div className="flex flex-col h-screen">
            <div className="flex flex-1 bg-muted/20 overflow-hidden flex-row bg-muted/20">
                <Outlet />
            </div >
            <div className="border-t bg-background">
                <div className="mx-auto w-full legacy-container px-6 py-4 flex items-center gap-4">
                    <Pagination>
                        <PaginationContent>
                            {steps.map((step, index) => (
                                <span key={step.name} className="flex items-center">
                                    <PaginationItem>
                                        <PaginationLink
                                            href={step.href}
                                            className={isStepActive(step.href) ? 'bg-accent text-accent-foreground' : ''}
                                            aria-label={`Go to ${step.name} step`}
                                            size="default"
                                        >
                                            {step.name}
                                        </PaginationLink>
                                    </PaginationItem>
                                    {index < steps.length - 1 && <ChevronRight strokeWidth={1} />}
                                </span>
                            ))}
                        </PaginationContent>
                    </Pagination>
                    {nextStep && (
                        <a href={nextStep.href}>
                            <Button variant="default">
                                Next
                            </Button>
                        </a>
                    )}
                </div>
            </div>
        </div >
    )
}
