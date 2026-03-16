import type { JourneyStepType } from "../../../types"
import { StickyStepIcon } from "../../../components/icons"
import { useTranslation } from "react-i18next"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
import TextAutoLink from "./TextAutoLink"

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
        return (
            <div className="space-y-1.5 h-full w-full">
                <Label className="text-sm font-medium">{t("sticky_text_label")}</Label>
                <Textarea
                    value={value.text ?? ""}
                    onChange={(e) => onChange({ ...value, text: e.target.value })}
                />
            </div>
        )
    },
    hideBottomHandle: true,
    hideTopHandle: true,
}
