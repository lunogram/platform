import type { JourneyStepType } from "../../../types"
import { BalancerStepIcon } from "../../../components/icons"
import { useTranslation } from "react-i18next"

export const balancerStep: JourneyStepType<Record<string, never>> = {
    name: "balancer",
    icon: <BalancerStepIcon />,
    category: "flow",
    description: "balancer_desc",
    Describe: () => {
        const { t } = useTranslation()
        return <>{t("balancer_desc_empty")}</>
    },
    multiChildSources: true,
}
