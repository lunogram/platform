import type { JourneyStepType } from "../../../types"
import { BalancerStepIcon } from "../../../components/icons"
import { useTranslation } from "react-i18next"
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Label } from "@/components/ui/label"
import { Input } from "@/components/ui/input"

interface BalancerStepChildConfig {
    rate_limit?: number
    rate_interval?: "second" | "minute" | "hour" | "day"
}

const intervals = ["second", "minute", "hour", "day"] as const

export const balancerStep: JourneyStepType<BalancerStepChildConfig> = {
    name: "balancer",
    icon: <BalancerStepIcon />,
    category: "flow",
    description: "balancer_desc",
    Describe: ({ value }) => {
        const { t } = useTranslation()
        if (!value.rate_limit) return <>{t("balancer_desc_empty")}</>
        return <div className="max-w-[300px]">{t("balancer_desc_values", { ...value })}</div>
    },
    newData: async () => ({
        rate_limit: 0,
        rate_interval: "minute",
    }),
    Edit: ({ onChange, value }) => {
        const { t } = useTranslation()
        return (
            <>
                <p className="text-sm text-muted-foreground">{t("balancer_edit_desc")}</p>
                <div className="space-y-1.5">
                    <Label className="text-sm font-medium">{t("period")}</Label>
                    <Tabs
                        value={value.rate_interval}
                        onValueChange={(interval) =>
                            onChange({
                                ...value,
                                rate_interval: interval as BalancerStepChildConfig["rate_interval"],
                            })
                        }
                    >
                        <TabsList className="w-full">
                            {intervals.map((key) => (
                                <TabsTrigger key={key} value={key} className="flex-1 capitalize">
                                    {t(key)}
                                </TabsTrigger>
                            ))}
                        </TabsList>
                    </Tabs>
                </div>
                <div className="space-y-1.5">
                    <Label className="text-sm font-medium">{t("rate_limit")}</Label>
                    <Input
                        type="number"
                        value={value.rate_limit}
                        onChange={(e) => onChange({ ...value, rate_limit: e.target.valueAsNumber })}
                    />
                </div>
            </>
        )
    },
    multiChildSources: true,
}
