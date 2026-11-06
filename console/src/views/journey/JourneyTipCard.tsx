import { useTranslation } from "react-i18next"
import { ArrowRight, GitBranch, Workflow, Zap } from "lucide-react"
import { Button } from "@/components/ui/button"

export function JourneyTipCard() {
    const { t } = useTranslation()
    return (
        <div className="group relative overflow-hidden rounded-lg bg-gradient-to-br from-primary/10 via-primary/5 to-transparent border p-6">
            <div className="relative z-10 max-w-md">
                <h3 className="font-semibold text-foreground">
                    {t("journey_tip_title", "Design automated journeys")}
                </h3>
                <p className="mt-1 text-sm text-muted-foreground">
                    {t(
                        "journey_tip_description",
                        "Create multi-step automated workflows to engage users at the right moment with the right message.",
                    )}
                </p>
                <Button
                    variant="link"
                    className="mt-2 h-auto p-0 text-primary"
                    onClick={() => window.open("/api/", "_blank")}
                >
                    {t("view_api_docs", "View API documentation")}
                    <ArrowRight className="ml-1 h-3 w-3 transition-transform duration-300 group-hover:translate-x-1" />
                </Button>
            </div>

            {/* Decorative elements */}
            <div className="absolute -right-6 -bottom-6 flex gap-4">
                <div className="hidden sm:flex h-20 w-20 items-center justify-center rounded-xl bg-primary/10 rotate-12 transition-all duration-500 ease-out group-hover:rotate-6 group-hover:-translate-y-2 group-hover:bg-primary/15">
                    <GitBranch
                        className="h-10 w-10 text-primary/40 transition-all duration-500 group-hover:text-primary/60 group-hover:scale-110"
                        strokeWidth={1.25}
                    />
                </div>
                <div className="flex h-20 w-20 items-center justify-center rounded-xl bg-primary/10 -rotate-6 translate-y-4 transition-all duration-500 ease-out delay-75 group-hover:rotate-3 group-hover:translate-y-0 group-hover:bg-primary/15">
                    <Workflow
                        className="h-10 w-10 text-primary/40 transition-all duration-500 delay-75 group-hover:text-primary/60 group-hover:scale-110"
                        strokeWidth={1.25}
                    />
                </div>
                <div className="flex h-20 w-20 items-center justify-center rounded-xl bg-primary/10 rotate-12 -translate-y-2 transition-all duration-500 ease-out delay-150 group-hover:-rotate-6 group-hover:-translate-y-4 group-hover:bg-primary/15">
                    <Zap
                        className="h-10 w-10 text-primary/40 transition-all duration-500 delay-150 group-hover:text-primary/60 group-hover:scale-110"
                        strokeWidth={1.25}
                    />
                </div>
            </div>
        </div>
    )
}
