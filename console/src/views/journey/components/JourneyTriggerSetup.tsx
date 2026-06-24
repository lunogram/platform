import {
    ArrowRight,
    CalendarClock,
    ListChecks,
    MousePointerClick,
    Webhook,
    Zap,
} from "lucide-react"
import { useTranslation } from "react-i18next"

import { cn } from "@/utils"

export type EntranceTrigger = "event" | "scheduled" | "list" | "none"

interface JourneyTriggerSetupProps {
    onSelectTrigger: (trigger: EntranceTrigger) => void
}

const triggerOptions: Array<{
    trigger: EntranceTrigger
    icon: typeof Zap
    titleKey: string
    titleFallback: string
    descriptionKey: string
    descriptionFallback: string
    className: string
}> = [
    {
        trigger: "event",
        icon: Zap,
        titleKey: "journey_trigger_event_title",
        titleFallback: "Start when an event happens",
        descriptionKey: "journey_trigger_event_desc",
        descriptionFallback: "Add users when they perform a tracked action, with optional filters.",
        className:
            "border-emerald-200 bg-emerald-50/70 text-emerald-700 dark:border-emerald-900 dark:bg-emerald-950/30 dark:text-emerald-300",
    },
    {
        trigger: "scheduled",
        icon: CalendarClock,
        titleKey: "journey_trigger_scheduled_title",
        titleFallback: "Start from a schedule",
        descriptionKey: "journey_trigger_scheduled_desc",
        descriptionFallback:
            "Use a scheduled moment, such as before or after a date on the user profile.",
        className:
            "border-amber-200 bg-amber-50/70 text-amber-700 dark:border-amber-900 dark:bg-amber-950/30 dark:text-amber-300",
    },
    {
        trigger: "list",
        icon: ListChecks,
        titleKey: "journey_trigger_list_title",
        titleFallback: "Start from a list",
        descriptionKey: "journey_trigger_list_desc",
        descriptionFallback:
            "Add users when they join a list, and optionally exit them when they leave it.",
        className:
            "border-violet-200 bg-violet-50/70 text-violet-700 dark:border-violet-900 dark:bg-violet-950/30 dark:text-violet-300",
    },
    {
        trigger: "none",
        icon: Webhook,
        titleKey: "journey_trigger_api_title",
        titleFallback: "Trigger from the API",
        descriptionKey: "journey_trigger_api_desc",
        descriptionFallback: "Create an API entrance and trigger users directly from your backend.",
        className:
            "border-blue-200 bg-blue-50/70 text-blue-700 dark:border-blue-900 dark:bg-blue-950/30 dark:text-blue-300",
    },
]

export function JourneyTriggerSetup({ onSelectTrigger }: JourneyTriggerSetupProps) {
    const { t } = useTranslation()

    return (
        <div className="relative flex min-h-0 flex-1 overflow-y-auto overflow-x-hidden bg-muted/20">
            <div className="absolute inset-0 bg-[radial-gradient(circle_at_20%_20%,hsl(var(--primary)/0.11),transparent_28%),radial-gradient(circle_at_78%_14%,hsl(var(--muted-foreground)/0.08),transparent_24%),linear-gradient(135deg,transparent_0%,hsl(var(--background))_70%)]" />
            <div className="relative mx-auto flex w-full max-w-6xl flex-col justify-center px-6 py-8 sm:px-10 lg:px-12">
                <div className="mb-8 max-w-2xl">
                    <div className="mb-4 inline-flex items-center gap-2 rounded-full border bg-background/80 px-3 py-1 text-xs font-medium text-muted-foreground shadow-sm backdrop-blur">
                        <MousePointerClick className="h-3.5 w-3.5" />
                        {t("journey_first_step", "First step")}
                    </div>
                    <h2 className="text-3xl font-semibold tracking-tight sm:text-4xl">
                        {t("journey_choose_trigger_title", "How should users enter this journey?")}
                    </h2>
                    <p className="mt-3 text-base text-muted-foreground sm:text-lg">
                        {t(
                            "journey_choose_trigger_desc",
                            "Every journey needs an entrance. Choose the trigger first, then configure the details before adding the rest of the flow.",
                        )}
                    </p>
                </div>

                <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
                    {triggerOptions.map((option) => {
                        const Icon = option.icon
                        return (
                            <button
                                key={option.trigger}
                                type="button"
                                onClick={() => onSelectTrigger(option.trigger)}
                                className="group flex min-h-56 cursor-pointer flex-col rounded-2xl border bg-background/90 p-5 text-left shadow-sm backdrop-blur transition-all duration-200 hover:-translate-y-1 hover:border-primary/40 hover:shadow-xl focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
                            >
                                <span
                                    className={cn(
                                        "mb-5 flex h-12 w-12 items-center justify-center rounded-xl border [&_svg]:h-5 [&_svg]:w-5",
                                        option.className,
                                    )}
                                >
                                    <Icon />
                                </span>
                                <span className="text-lg font-semibold text-foreground">
                                    {t(option.titleKey, option.titleFallback)}
                                </span>
                                <span className="mt-2 flex-1 text-sm leading-6 text-muted-foreground">
                                    {t(option.descriptionKey, option.descriptionFallback)}
                                </span>
                                <span className="mt-5 inline-flex items-center gap-2 text-sm font-medium text-primary">
                                    {t("configure_trigger", "Configure trigger")}
                                    <ArrowRight className="h-4 w-4 transition-transform group-hover:translate-x-1" />
                                </span>
                            </button>
                        )
                    })}
                </div>

                <div className="mt-6 rounded-xl border border-dashed bg-background/70 px-4 py-3 text-sm text-muted-foreground backdrop-blur">
                    {t(
                        "journey_trigger_setup_hint",
                        "Tip: you can add more entrances later from the Components panel.",
                    )}
                </div>
            </div>
        </div>
    )
}
