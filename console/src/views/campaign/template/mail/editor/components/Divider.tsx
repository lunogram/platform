import type { ComponentConfig } from "@puckeditor/core"
import { Layout, type LayoutProps, layoutClassMap } from "./fields/Layout"
import { cn } from "@/utils"
import { Spacing, type SpacingProps, spacingClassMap } from "./fields/Spacing"
import { Decoration, type DecorationProps, decorationClassMap } from "./fields/Decoration"
import { generateTailwindClasses } from "./fields/unit"

export interface DividerProps {
    layout: LayoutProps
    spacing: SpacingProps
    decoration: DecorationProps
}

export const Divider: ComponentConfig<DividerProps> = {
    fields: {
        layout: Layout,
        spacing: Spacing,
        decoration: Decoration,
    },
    defaultProps: {
        layout: {
            xl: {
                width: "100%",
            },
        },
        spacing: {
            xl: {
                marginTop: "16",
                marginBottom: "16",
            },
        },
        decoration: {
            xl: {
                borderTopWidth: "1",
                borderWidthLinked: false,
                borderColor: "#d1d5db",
                borderStyle: "solid",
            },
        },
    },
    render: ({ layout, spacing, decoration }) => {
        const classes = cn(
            generateTailwindClasses(layout, layoutClassMap),
            generateTailwindClasses(spacing, spacingClassMap),
            generateTailwindClasses(decoration, decorationClassMap),
        )

        return (
            // The React.email Hr element contains styles that conflict with our
            // Tailwind styles, so we use a plain hr element here.
            // https://github.com/resend/react-email/blob/8531d42850b007babf7486c52a84c9eebacfd589/packages/hr/src/hr.tsx
            <hr className={classes} />
        )
    },
}
