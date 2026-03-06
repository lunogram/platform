import { Markdown as EmailMarkdown } from "@react-email/components"
import type { ComponentConfig } from "@puckeditor/core"
import { Typography, type TypographyProps, typographyClassMap } from "./fields/Typography"
import { Spacing, type SpacingProps, spacingClassMap } from "./fields/Spacing"
import { generateTailwindClasses } from "./fields/unit"
import { cn } from "@/utils"

export interface MarkdownProps {
    content: string
    typography: TypographyProps
    spacing: SpacingProps
}

export const Markdown: ComponentConfig<MarkdownProps> = {
    fields: {
        content: { type: "textarea" },
        typography: Typography,
        spacing: Spacing,
    },
    defaultProps: {
        content: "# Hello World\nThis is **markdown** content.",
        typography: {},
        spacing: { xl: { paddingTop: "16", paddingBottom: "16" } },
    },
    render: ({ content, typography, spacing }) => {
        const classes = cn(
            generateTailwindClasses(typography, typographyClassMap),
            generateTailwindClasses(spacing, spacingClassMap),
        )
        return (
            <div className={classes}>
                <EmailMarkdown children={content} />
            </div>
        )
    },
}
