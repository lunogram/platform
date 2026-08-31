import { useCallback, useMemo } from "react"
import { useTranslation } from "react-i18next"
import { Palette, Plus, Trash2 } from "lucide-react"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import type { CampaignVariant, CampaignVariants as CampaignVariantsValue, Template } from "@/types"

import { VariantSelectorInput } from "./VariantSelectorInput"

interface CampaignVariantsProps {
    value: CampaignVariantsValue
    templates: Template[]
    onChange: (variants: CampaignVariantsValue) => void
}

const VARIANT_KEY_REGEX = /^[a-z0-9][a-z0-9_-]*$/

function validateKey(key: string, variants: CampaignVariant[], index: number): string | undefined {
    if (!key) return undefined
    if (!VARIANT_KEY_REGEX.test(key))
        return "Must be lowercase letters, numbers, dashes, underscores"
    if (variants.findIndex((v, i) => i !== index && v.key === key) !== -1) return "Duplicate key"
    return undefined
}

export function CampaignVariants({ value, templates, onChange }: CampaignVariantsProps) {
    const { t } = useTranslation()

    const variants = useMemo(() => value.options ?? [], [value.options])

    const setOptions = useCallback(
        (options: CampaignVariant[]) => {
            // Dropping the variant a static selector points at would leave the
            // campaign pinned to something it no longer declares, which the API
            // rejects on save. Clear the selector instead.
            const selector =
                value.selector?.type === "static" &&
                !options.some((option) => option.key === value.selector?.key)
                    ? undefined
                    : value.selector
            onChange({ selector, options })
        },
        [value.selector, onChange],
    )

    const addVariant = useCallback(() => {
        setOptions([...variants, { key: "" }])
    }, [variants, setOptions])

    const updateVariant = useCallback(
        (index: number, updates: Partial<CampaignVariant>) => {
            setOptions(variants.map((v, i) => (i === index ? { ...v, ...updates } : v)))
        },
        [variants, setOptions],
    )

    const removeVariant = useCallback(
        (index: number) => {
            setOptions(variants.filter((_, i) => i !== index))
        },
        [variants, setOptions],
    )

    const editor =
        variants.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-12">
                <div className="mb-3 flex h-10 w-10 items-center justify-center rounded-full bg-muted">
                    <Palette className="h-5 w-5 text-muted-foreground" />
                </div>
                <p className="mb-1 text-sm font-medium">
                    {t("campaign.variants.empty_title", "No variants yet")}
                </p>
                <p className="mb-4 max-w-xs text-center text-xs text-muted-foreground">
                    {t(
                        "campaign.variants.empty_description",
                        "Add one when a client needs this campaign in their own design or wording.",
                    )}
                </p>
                <Button
                    variant="outline"
                    size="sm"
                    onClick={addVariant}
                    className="shadow-none"
                    type="button"
                >
                    <Plus className="mr-2 h-4 w-4" />
                    {t("campaign.variants.add", "Add variant")}
                </Button>
            </div>
        ) : (
            <div className="px-3 py-2">
                {variants.map((variant, index) => {
                    const keyError = validateKey(variant.key, variants, index)
                    const templateCount = templates.filter((t) => t.variant === variant.key).length

                    return (
                        <div key={index} className="py-1.5">
                            <div className="inline-flex w-full min-w-0 items-center">
                                <Input
                                    value={variant.key}
                                    onChange={(e) =>
                                        updateVariant(index, {
                                            key: e.target.value.toLowerCase().replace(/\s/g, "-"),
                                        })
                                    }
                                    placeholder="key"
                                    className="h-8 w-36 rounded-r-none font-mono text-sm shadow-none focus:z-10"
                                />
                                <Input
                                    value={variant.label ?? ""}
                                    onChange={(e) =>
                                        updateVariant(index, {
                                            label: e.target.value || undefined,
                                        })
                                    }
                                    placeholder="display name (optional)"
                                    className="-ml-px h-8 min-w-[120px] flex-1 rounded-none border-l-0 text-sm shadow-none focus:z-10"
                                />
                                <button
                                    type="button"
                                    onClick={() => removeVariant(index)}
                                    className="-ml-px flex h-8 w-8 shrink-0 cursor-pointer items-center justify-center rounded-r-md border border-l-0 bg-background text-muted-foreground/60 transition-colors hover:bg-destructive/5 hover:text-destructive"
                                    aria-label={t("campaign.variants.delete", "Delete variant")}
                                >
                                    <Trash2 className="h-3.5 w-3.5" />
                                </button>
                            </div>
                            {keyError && variant.key && (
                                <p className="ml-1 mt-1 text-xs text-destructive">{keyError}</p>
                            )}
                            {!keyError && variant.key && templateCount === 0 && (
                                <p className="ml-1 mt-1 text-xs text-amber-600 dark:text-amber-500">
                                    {t(
                                        "campaign.variants.no_template",
                                        "No template yet — sends for this variant fall back to the default design.",
                                    )}
                                </p>
                            )}
                        </div>
                    )
                })}

                <div className="flex items-center py-1.5">
                    <Button
                        variant="outline"
                        size="sm"
                        onClick={addVariant}
                        className="h-8 text-xs shadow-none"
                        type="button"
                    >
                        <Plus className="mr-1.5 h-3.5 w-3.5" />
                        {t("campaign.variants.add", "Add variant")}
                    </Button>
                </div>
            </div>
        )

    return (
        <div className="overflow-hidden rounded-lg border">
            <div className="flex items-center justify-between gap-3 border-b bg-muted/30 px-3 py-2">
                <span className="text-sm font-medium text-muted-foreground">
                    {t("campaign.variants.title", "Variants")}
                </span>
            </div>
            {editor}
            {variants.length > 0 && (
                <div className="space-y-1.5 border-t bg-muted/50 px-3 py-3 shadow-inner">
                    <Label className="text-xs font-medium">
                        {t("campaign.variants.selector.title", "Which variant to send")}
                    </Label>
                    <p className="pb-1 text-xs text-muted-foreground">
                        {t(
                            "campaign.variants.selector.description",
                            "Used when a journey step or broadcast does not pick a variant itself.",
                        )}
                    </p>
                    <VariantSelectorInput
                        value={value.selector}
                        options={variants.filter((variant) => variant.key)}
                        onChange={(selector) => onChange({ ...value, selector })}
                    />
                </div>
            )}
        </div>
    )
}
