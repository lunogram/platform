import type { JourneyStepType } from "../../../types"
import { StickyStepIcon } from "../../../components/icons"
import { useTranslation } from "react-i18next"
import { Label } from "@/components/ui/label"
import TextAutoLink from "./TextAutoLink"
import { useRef, useEffect } from "react"

interface StickyConfig {
    text?: string
}

export const stickyStep: JourneyStepType<StickyConfig> = {
    name: "sticky",
    icon: <StickyStepIcon />,
    category: "info",
    description: "sticky_desc",
    Describe({ value }) {
        return (
            <div>
                <TextAutoLink text={value.text ?? ""} />
            </div>
        )
    },
    Edit({ onChange, value }) {
        const { t } = useTranslation()
        const textareaRef = useRef<HTMLTextAreaElement>(null)

        useEffect(() => {
            const textarea = textareaRef.current
            if (!textarea) return

            const adjustHeight = () => {
                const maxHeight = 1000
                textarea.style.height = "auto"
                const newHeight = Math.min(textarea.scrollHeight, maxHeight)
                textarea.style.height = `${newHeight}px`
            }

            adjustHeight()
        }, [value.text])

        return (
            <div className="space-y-1.5 w-full">
                <Label className="text-sm font-medium">{t("sticky_text_label")}</Label>
                <textarea
                    ref={textareaRef}
                    value={value.text ?? ""}
                    onChange={(e) => onChange({ ...value, text: e.target.value })}
                    className="min-h-[100px] max-h-[1000px] w-full resize-none rounded-md border border-input bg-transparent px-3 py-2 text-sm shadow-xs placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-ring/50 outline-none focus-visible:ring-[3px] overflow-y-auto"
                />
            </div>
        )
    },
    hideBottomHandle: true,
    hideTopHandle: true,
}
