import type { ComponentConfig, Slot } from "@puckeditor/core"
import { Row as EmailRow, Column as EmailColumn } from "@react-email/components"
import { Layout, type LayoutProps, layoutClassMap } from "./fields/Layout"
import { cn } from "@/utils"
import { Spacing, type SpacingProps, spacingClassMap } from "./fields/Spacing"
import { Typography, type TypographyProps, typographyClassMap } from "./fields/Typography"
import { Decoration, type DecorationProps, decorationClassMap } from "./fields/Decoration"
import { generateTailwindClasses } from "./fields/unit"

export interface ColumnProps {
    align: "left" | "center" | "right"
    content: Slot
    layout: LayoutProps
    spacing: SpacingProps
    typography: TypographyProps
    decoration: DecorationProps
}

export const Column: ComponentConfig<ColumnProps> = {
    fields: {
        align: {
            type: "select",
            options: [
                { label: "Left", value: "left" },
                { label: "Center", value: "center" },
                { label: "Right", value: "right" },
            ],
        },
        content: {
            type: "slot",
        },
        layout: Layout,
        typography: Typography,
        spacing: Spacing,
        decoration: Decoration,
    },
    defaultProps: {
        align: "left",
        content: [],
        layout: {
            xl: {
                width: "33.33%",
            },
        },
        typography: {},
        spacing: {},
        decoration: {},
    },
    render: ({ content: Content, align, layout, spacing, typography, decoration }) => {
        const classes = cn(
            "border-separate",
            generateTailwindClasses(layout, layoutClassMap),
            generateTailwindClasses(spacing, spacingClassMap),
            generateTailwindClasses(typography, typographyClassMap),
            generateTailwindClasses(decoration, decorationClassMap),
        )

        return (
            <EmailRow align={align} className={classes}>
                <EmailColumn>
                    <Content />
                </EmailColumn>
            </EmailRow>
        )
    },
}
