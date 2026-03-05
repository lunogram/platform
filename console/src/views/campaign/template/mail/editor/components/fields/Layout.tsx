/* eslint-disable react-hooks/rules-of-hooks */
import { usePuck, type CustomField } from "@puckeditor/core"
import { getViewportTailwindBreakpoint } from "../../viewport"
import { addUnit } from "./unit"
import { useTranslation } from "react-i18next"

import { Input } from "@/components/ui/input"

export interface LayoutViewport {
    width: string
    minWidth: string
    maxWidth: string
    height: string
    minHeight: string
    maxHeight: string
}

export interface LayoutProps {
    sm?: Partial<LayoutViewport>
    md?: Partial<LayoutViewport>
    xl?: Partial<LayoutViewport>
}

export const layoutClassMap: Record<
    keyof LayoutViewport,
    (value: string, prefix: string) => string
> = {
    width: (value, prefix) => `${prefix}w-${addUnit(value)}`,
    minWidth: (value, prefix) => `${prefix}min-w-${addUnit(value)}`,
    maxWidth: (value, prefix) => `${prefix}max-w-${addUnit(value)}`,
    height: (value, prefix) => `${prefix}h-${addUnit(value)}`,
    minHeight: (value, prefix) => `${prefix}min-h-${addUnit(value)}`,
    maxHeight: (value, prefix) => `${prefix}max-h-${addUnit(value)}`,
}

const maxBreakpointWidth: number = 1280

export const Layout: CustomField<LayoutProps> = {
    type: "custom",
    render: ({ onChange, value = {} }) => {
        const { t } = useTranslation()

        const { appState } = usePuck()
        const viewport = appState.ui.viewports.current
        const breakpoint = getViewportTailwindBreakpoint(
            typeof viewport.width == "number" ? viewport.width : maxBreakpointWidth,
        )

        const config = value[breakpoint] || {}

        const handleChange = (field: string, val: string) => {
            onChange({
                ...value,
                [breakpoint]: {
                    ...config,
                    [field]: val,
                },
            })
        }

        const fields: Array<{
            key: keyof LayoutViewport
            label: string
            placeholder: string
        }> = [
            {
                key: "width",
                label: t("editor.fields.layout.width"),
                placeholder: t("editor.fields.layout.width_placeholder"),
            },
            {
                key: "height",
                label: t("editor.fields.layout.height"),
                placeholder: t("editor.fields.layout.height_placeholder"),
            },
            {
                key: "minWidth",
                label: t("editor.fields.layout.min_width"),
                placeholder: t("editor.fields.layout.min_width_placeholder"),
            },
            {
                key: "minHeight",
                label: t("editor.fields.layout.min_height"),
                placeholder: t("editor.fields.layout.min_height_placeholder"),
            },
            {
                key: "maxWidth",
                label: t("editor.fields.layout.max_width"),
                placeholder: t("editor.fields.layout.max_width_placeholder"),
            },
            {
                key: "maxHeight",
                label: t("editor.fields.layout.max_height"),
                placeholder: t("editor.fields.layout.max_height_placeholder"),
            },
        ]

        return (
            <div className="space-y-3">
                <div className="grid grid-cols-2 gap-2">
                    {fields.map((field) => (
                        <div key={field.key} className="space-y-1">
                            <label className="text-xs font-medium text-gray-600 uppercase tracking-wide">
                                {field.label}
                            </label>
                            <Input
                                value={config[field.key] ?? ""}
                                onChange={(e) => handleChange(field.key, e.target.value)}
                                placeholder={field.placeholder}
                                className="h-8 text-sm"
                            />
                        </div>
                    ))}
                </div>
            </div>
        )
    },
}
