import FilterRuleEdit from "./FilterRuleEdit"
import JourneyRuleEdit from "./JourneyRuleEdit"
import type { RuleEditProps } from "./RuleHelpers"
import { isStepVisitRule, isWrapper } from "./RuleHelpers"
import StepVisitRuleEdit from "./StepVisitRuleEdit"
import WrapperRuleEdit from "./WrapperRuleEdit"

export default function RuleEdit({ rule, setRule, ...props }: RuleEditProps) {
    if (isWrapper(rule)) {
        return <WrapperRuleEdit rule={rule} setRule={setRule} {...props} />
    }

    if (isStepVisitRule(rule)) {
        return <StepVisitRuleEdit rule={rule} setRule={setRule} {...props} />
    }

    if (rule?.group === "journey") {
        return <JourneyRuleEdit rule={rule} setRule={setRule} {...props} />
    }

    return <FilterRuleEdit rule={rule} setRule={setRule} {...props} />
}
