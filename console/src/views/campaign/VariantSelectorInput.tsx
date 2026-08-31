import { useCallback } from "react"
import { useTranslation } from "react-i18next"

import { Input } from "@/components/ui/input"
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select"
import { TemplateInput } from "@/components/ui/template-input"
import type { CampaignVariant, VariantSelector, VariantSelectorType } from "@/types"

const NONE = "__none__"

interface VariantSelectorInputProps {
    value?: VariantSelector
    options: CampaignVariant[]
    onChange: (selector: VariantSelector | undefined) => void
    /** Liquid variables offered by the surrounding context, when there are any. */
    variables?: React.ComponentProps<typeof TemplateInput>["variables"]
    /** Label for the "no selector" choice, which differs per surface. */
    emptyLabel?: string
}

/**
 * Picks a template variant either by pinning one or by writing a Liquid
 * expression resolved per recipient. Shared by the campaign, the journey
 * campaign step and the broadcast dialog so the three offer the same choices.
 */
export function VariantSelectorInput({
    value,
    options,
    onChange,
    variables,
    emptyLabel,
}: VariantSelectorInputProps) {
    const { t } = useTranslation()

    const mode: string = value?.type ?? NONE

    const handleModeChange = useCallback(
        (next: string) => {
            if (next === NONE) return onChange(undefined)
            onChange(
                next === "static"
                    ? { type: "static", key: options[0]?.key ?? "" }
                    : { type: "expression", expression: "" },
            )
        },
        [onChange, options],
    )

    return (
        <div className="space-y-1.5">
            <Select value={mode} onValueChange={handleModeChange}>
                <SelectTrigger className="h-9">
                    <SelectValue />
                </SelectTrigger>
                <SelectContent>
                    <SelectItem value={NONE}>
                        {emptyLabel ?? t("campaign.variants.selector.none", "Default variant")}
                    </SelectItem>
                    <SelectItem value="static">
                        {t("campaign.variants.selector.static", "Always one variant")}
                    </SelectItem>
                    <SelectItem value="expression">
                        {t("campaign.variants.selector.expression", "Decide per recipient")}
                    </SelectItem>
                </SelectContent>
            </Select>

            {value?.type === "static" && (
                <Select
                    value={value.key ?? ""}
                    onValueChange={(key: string) => onChange({ type: "static", key })}
                >
                    <SelectTrigger className="h-9">
                        <SelectValue
                            placeholder={t("campaign.variants.selector.pick", "Pick a variant")}
                        />
                    </SelectTrigger>
                    <SelectContent>
                        {options.map((option) => (
                            <SelectItem key={option.key} value={option.key}>
                                {option.label || option.key}
                            </SelectItem>
                        ))}
                    </SelectContent>
                </Select>
            )}

            {value?.type === "expression" &&
                (variables ? (
                    <TemplateInput
                        value={value.expression ?? ""}
                        onChange={(expression: string) =>
                            onChange({ type: "expression", expression })
                        }
                        variables={variables}
                        placeholder="{{ user.data.tenant }}"
                    />
                ) : (
                    <Input
                        value={value.expression ?? ""}
                        onChange={(e) =>
                            onChange({ type: "expression", expression: e.target.value })
                        }
                        placeholder="{{ user.data.tenant }}"
                        className="h-9 font-mono text-sm shadow-none"
                    />
                ))}

            {value?.type === "expression" && (
                <p className="text-xs text-muted-foreground">
                    {t(
                        "campaign.variants.selector.expression_help",
                        "A value matching no variant falls back to the default design.",
                    )}
                </p>
            )}
        </div>
    )
}

export type { VariantSelectorType }
