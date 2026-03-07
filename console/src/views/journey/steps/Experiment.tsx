import type { JourneyStepType } from "../../../types"
import { ExperimentStepIcon } from "../../../components/icons"
import { round } from "../../../utils"
import { useTranslation } from "react-i18next"
import { Label } from "@/components/ui/label"
import { Input } from "@/components/ui/input"

interface ExperimentStepChildConfig {
    ratio: number
}

export const experimentStep: JourneyStepType<object, ExperimentStepChildConfig> = {
    name: "experiment",
    icon: <ExperimentStepIcon />,
    category: "flow",
    description: "experiment_desc",
    Describe: () => {
        const { t } = useTranslation()
        return <>{t("experiment_default")}</>
    },
    newEdgeData: async () => ({ ratio: 1 }),
    Edit: () => {
        const { t } = useTranslation()
        return (
            <p className="max-w-[300px] text-sm text-muted-foreground">
                {t("experiment_edit_desc")}
            </p>
        )
    },
    EditEdge({ siblingData, onChange, value }) {
        const { t } = useTranslation()
        const ratio = value.ratio ?? 0
        const totalRatio = siblingData.reduce((a, c) => a + c.ratio, ratio)
        const percentage = totalRatio > 0 ? round((ratio / totalRatio) * 100, 2) : 0
        return (
            <div className="space-y-1.5">
                <Label className="text-sm font-medium">{t("ratio")}</Label>
                <p className="text-xs text-muted-foreground">
                    {t("experiment_ratio_desc", { percentage })}
                </p>
                <Input
                    type="number"
                    value={value.ratio ?? 0}
                    onChange={(e) => onChange({ ...value, ratio: e.target.valueAsNumber })}
                />
            </div>
        )
    },
    multiChildSources: true,
}
