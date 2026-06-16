import type { JourneyStepType } from "../../../types"
import { CloseIcon } from "../../../components/icons"
import { useTranslation } from "react-i18next"
import { Label } from "@/components/ui/label"
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select"
import { snakeToTitle } from "../../../utils"
import type { Node } from "@xyflow/react"
import { useNodes } from "@xyflow/react"

interface ExitConfig {
    entrance_uuid?: string
}

const entranceName = ({ data: { type, name, data_key } }: Node) => {
    const stepName = name || snakeToTitle(type)
    return data_key ? `${stepName} (${data_key})` : stepName
}

export const exitStep: JourneyStepType<ExitConfig> = {
    name: "exit",
    icon: <CloseIcon />,
    category: "exit",
    description: "exit_desc",
    Describe({ value }) {
        const { t } = useTranslation()
        const nodes = useNodes()
        if (!value.entrance_uuid) return <></>
        const node = nodes.find((n) => n.id === value.entrance_uuid)
        if (!node) return <></>
        return (
            <div className="max-w-[300px] text-sm text-muted-foreground">
                {t("exit_step_default", { name: entranceName(node) })}
            </div>
        )
    },
    Edit({ onChange, value, nodes }) {
        const { t } = useTranslation()
        const steps = nodes
            .filter((item) => item.data.type === "entrance")
            .map((node) => ({ id: node.id, label: entranceName(node) }))
        return (
            <div className="space-y-1.5">
                <Label className="inline-flex items-center gap-0.5 text-sm font-medium">
                    {t("exit_entrance_label")}
                    <span className="text-destructive">*</span>
                </Label>
                <p className="text-xs text-muted-foreground">{t("exit_entrance_desc")}</p>
                <Select
                    value={value.entrance_uuid ?? ""}
                    onValueChange={(entrance_uuid) => onChange({ entrance_uuid })}
                >
                    <SelectTrigger>
                        <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                        {steps.map((step) => (
                            <SelectItem key={step.id} value={step.id}>
                                {step.label}
                            </SelectItem>
                        ))}
                    </SelectContent>
                </Select>
            </div>
        )
    },
    validate: ({ entrance_uuid }) => {
        return !!entrance_uuid
    },
    hideBottomHandle: true,
}
