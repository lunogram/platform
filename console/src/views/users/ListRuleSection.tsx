import { useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { RefreshCw } from "lucide-react"
import type { DynamicList, Rule } from "../../types"
import RuleBuilder from "./rules/RuleBuilder"
import { Button } from "@/components/ui/button"

interface ListRuleSectionProps {
    list: DynamicList
    isSaving: boolean
    /** @returns the saved list, or null when the save failed. */
    onRuleSave: (rule: Rule) => Promise<DynamicList | null>
    /** Called with whether the rule has edits that are not saved yet. */
    onDirtyChange?: (dirty: boolean) => void
}

/** The rule editor of a dynamic list. Edits stay local until saved. */
export function ListRuleSection({
    list,
    isSaving,
    onRuleSave,
    onDirtyChange,
}: ListRuleSectionProps) {
    const { t } = useTranslation()
    // Display draft_rule if available, otherwise fall back to published rule
    const displayRule = list.draft_rule ?? list.rule
    const [rule, setRule] = useState<Rule | null>(displayRule)
    const [hasLocalEdits, setHasLocalEdits] = useState(false)
    const onSetRule = (rule: Rule) => {
        setRule(rule)
        setHasLocalEdits(true)
        onDirtyChange?.(true)
    }

    // The latest rule, to tell whether it was edited again while a save ran.
    const ruleRef = useRef(rule)
    ruleRef.current = rule

    const save = async () => {
        if (!rule) return
        const saved = await onRuleSave(rule)
        // A failed save keeps the edits, and the page keeps blocking navigation.
        if (!saved || ruleRef.current !== rule) return
        setRule(saved.draft_rule ?? saved.rule)
        setHasLocalEdits(false)
        onDirtyChange?.(false)
    }

    return (
        <div className="space-y-4">
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
                <div>
                    <h3 className="text-base font-semibold">{t("rules")}</h3>
                    <p className="text-sm text-muted-foreground">
                        {t(
                            "rules_description",
                            "Define conditions to dynamically include users in this list.",
                        )}
                    </p>
                </div>
                <div className="flex items-center gap-2">
                    {hasLocalEdits && (
                        <span className="text-xs text-amber-600 dark:text-amber-500">
                            {t("unsaved_changes", "Unsaved changes")}
                        </span>
                    )}
                    <Button size="sm" onClick={save} disabled={isSaving || !rule}>
                        {isSaving ? <RefreshCw className="mr-2 h-3.5 w-3.5 animate-spin" /> : null}
                        {t("rules_save")}
                    </Button>
                </div>
            </div>
            <div className="rounded-lg border bg-card p-4">
                {rule && <RuleBuilder rule={rule} setRule={onSetRule} />}
            </div>
        </div>
    )
}
