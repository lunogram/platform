import type { ComponentConfig } from "@puckeditor/core"
import { Layout, type LayoutProps, layoutClassMap } from "./fields/Layout"
import { cn } from "@/utils"
import { Spacing, type SpacingProps, spacingClassMap } from "./fields/Spacing"
import { Typography, type TypographyProps, typographyClassMap } from "./fields/Typography"
import { Decoration, type DecorationProps, decorationClassMap } from "./fields/Decoration"
import { generateTailwindClasses } from "./fields/unit"

export interface TextSectionProps {
    value: string
    layout: LayoutProps
    spacing: SpacingProps
    typography: TypographyProps
    decoration: DecorationProps
}

export const TextSection: ComponentConfig<TextSectionProps> = {
    fields: {
        value: {
            type: "text",
            contentEditable: true,
        },
        layout: Layout,
        spacing: Spacing,
        typography: Typography,
        decoration: Decoration,
    },
    defaultProps: {
        value: "Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam.",
        layout: {},
        spacing: {},
        typography: {},
        decoration: {},
    },
    render: ({ value, layout, spacing, typography, decoration }) => {
        const classes = cn(
            generateTailwindClasses(layout, layoutClassMap),
            generateTailwindClasses(spacing, spacingClassMap),
            generateTailwindClasses(typography, typographyClassMap),
            generateTailwindClasses(decoration, decorationClassMap),
        )

        return <span className={classes}>{value}</span>
    },
}
