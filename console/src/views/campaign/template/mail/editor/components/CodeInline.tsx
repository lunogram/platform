import { CodeInline as EmailCodeInline } from "@react-email/components"
import type { ComponentConfig } from "@puckeditor/core"
import { Typography, type TypographyProps, typographyClassMap } from "./fields/Typography"
import { Spacing, type SpacingProps, spacingClassMap } from "./fields/Spacing"
import { Decoration, type DecorationProps, decorationClassMap } from "./fields/Decoration"
import { generateTailwindClasses } from "./fields/unit"
import { cn } from "@/utils"

export interface CodeInlineProps {
    text: string
    typography: TypographyProps
    spacing: SpacingProps
    decoration: DecorationProps
}

export const CodeInline: ComponentConfig<CodeInlineProps> = {
    fields: {
        text: { type: "text" },
        typography: Typography,
        spacing: Spacing,
        decoration: Decoration,
    },
    defaultProps: {
        text: "npm install",
        typography: {
            xl: {
                color: "#eb5757",
                fontFamily: "mono",
                fontSize: "14",
            },
        },
        decoration: {
            xl: {
                backgroundColor: "#f3f4f6",
                borderTopWidth: "1",
                borderBottomWidth: "1",
                borderLeftWidth: "1",
                borderRightWidth: "1",
                borderColor: "#e5e7eb",
                borderStyle: "solid",
                borderTopLeftRadius: "4",
                borderTopRightRadius: "4",
                borderBottomLeftRadius: "4",
                borderBottomRightRadius: "4",
            },
        },
        spacing: {
            xl: {
                paddingLeft: "4",
                paddingRight: "4",
                paddingTop: "2",
                paddingBottom: "2",
            },
        },
    },
    render: ({ text, typography, spacing, decoration }) => {
        const classes = cn(
            "inline-block border-separate",
            generateTailwindClasses(typography, typographyClassMap),
            generateTailwindClasses(spacing, spacingClassMap),
            generateTailwindClasses(decoration, decorationClassMap),
        )

        return <EmailCodeInline className={classes}>{text}</EmailCodeInline>
    },
}
