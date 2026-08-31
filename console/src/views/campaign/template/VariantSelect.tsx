import { useMemo } from "react"
import { useTranslation } from "react-i18next"
import { Palette } from "lucide-react"

import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select"
import type { CampaignVariant } from "@/types"

export const DEFAULT_VARIANT_VALUE = "__default__"

interface VariantSelectProps {
    variants: CampaignVariant[]
    value: string
    onChange: (variant: string) => void
    disabled?: boolean
}

export function VariantSelect({ variants, value, onChange, disabled }: VariantSelectProps) {
    const { t } = useTranslation()

    // The default variant is not a declared entry, so it gets a sentinel value:
    // Radix treats an empty string as "nothing selected" and would render the
    // placeholder instead of the option.
    const options = useMemo(
        () => [
            { value: DEFAULT_VARIANT_VALUE, label: t("campaign.variants.default", "Default") },
            ...variants
                .filter((variant) => variant.key)
                .map((variant) => ({
                    value: variant.key,
                    label: variant.label || variant.key,
                })),
        ],
        [variants, t],
    )

    return (
        <Select
            value={value || DEFAULT_VARIANT_VALUE}
            onValueChange={(next) => onChange(next === DEFAULT_VARIANT_VALUE ? "" : next)}
            disabled={disabled}
        >
            <SelectTrigger
                className="h-9 w-[180px]"
                aria-label={t("campaign.variants.title", "Variants")}
            >
                <span className="flex min-w-0 items-center gap-2">
                    <Palette className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                    <SelectValue />
                </span>
            </SelectTrigger>
            <SelectContent>
                {options.map((option) => (
                    <SelectItem key={option.value} value={option.value}>
                        {option.label}
                    </SelectItem>
                ))}
            </SelectContent>
        </Select>
    )
}
