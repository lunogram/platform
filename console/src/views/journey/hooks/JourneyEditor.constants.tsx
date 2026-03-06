import type { ReactNode } from "react"
import {
    ActionStepIcon,
    CheckCircleIcon,
    CloseIcon,
    DelayStepIcon,
    EntranceStepIcon,
    ForbiddenIcon,
} from "@/components/icons"

export const DATA_FORMAT = "application/lunogram-journey-step"
export const STEP_STYLE = "smoothstep"

export const statIcons: Record<string, ReactNode> = {
    campaign: <ActionStepIcon />,
    delay: <DelayStepIcon />,
    completed: <CheckCircleIcon />,
    error: <ForbiddenIcon />,
    entrance: <EntranceStepIcon />,
    ended: <CloseIcon />,
}

export const stepCategoryColors: Record<string, string> = {
    entrance: "bg-red-100 text-red-600 dark:bg-red-950 dark:text-red-400",
    action: "bg-blue-100 text-blue-600 dark:bg-blue-950 dark:text-blue-400",
    flow: "bg-emerald-100 text-emerald-600 dark:bg-emerald-950 dark:text-emerald-400",
    delay: "bg-amber-100 text-amber-600 dark:bg-amber-950 dark:text-amber-400",
    exit: "bg-red-100 text-red-600 dark:bg-red-950 dark:text-red-400",
    info: "bg-purple-100 text-purple-600 dark:bg-purple-950 dark:text-purple-400",
}

export const stepCategoryBorderColors: Record<string, string> = {
    entrance: "border-red-300 dark:border-red-800",
    action: "border-blue-300 dark:border-blue-800",
    flow: "border-emerald-300 dark:border-emerald-800",
    delay: "border-amber-300 dark:border-amber-800",
    exit: "border-red-300 dark:border-red-800",
    info: "border-purple-300 dark:border-purple-800",
}

export const stepCategoryMinimap: Record<string, string> = {
    entrance: "fill-red-200 dark:fill-red-900",
    action: "fill-blue-200 dark:fill-blue-900",
    flow: "fill-emerald-200 dark:fill-emerald-900",
    delay: "fill-amber-200 dark:fill-amber-900",
    exit: "fill-red-200 dark:fill-red-900",
    info: "fill-purple-200 dark:fill-purple-900",
}
