import { Heading as EmailHeading } from "@react-email/components"
import { type ComponentConfig } from "@puckeditor/core"
import { Typography, type TypographyProps, typographyClassMap } from "./fields/Typography"
import { Spacing, type SpacingProps, spacingClassMap } from "./fields/Spacing"
import { generateTailwindClasses } from "./fields/unit"
import { cn } from "@/utils"

export interface HeadingProps {
    text: string
    as: "h1" | "h2" | "h3"
    typography: TypographyProps
    spacing: SpacingProps
}

export const Heading: ComponentConfig<HeadingProps> = {
    fields: {
        text: { type: "text" },
        as: {
            type: "select",
            options: [
                { label: "H1", value: "h1" },
                { label: "H2", value: "h2" },
                { label: "H3", value: "h3" },
            ],
        },
        typography: Typography,
        spacing: Spacing,
    },
    defaultProps: {
        text: "Heading Title",
        as: "h1",
        typography: {},
        spacing: {},
    },
    render: ({ text, as, typography, spacing }) => {
        const classes = cn(
            generateTailwindClasses(typography, typographyClassMap),
            generateTailwindClasses(spacing, spacingClassMap),
        )
        return (
            <EmailHeading as={as} className={classes}>
                {text}
            </EmailHeading>
        )
    },
}
