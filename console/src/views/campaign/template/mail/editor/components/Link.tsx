import { Link as EmailLink } from "@react-email/components"
import { type ComponentConfig } from "@puckeditor/core"
import { Typography, type TypographyProps, typographyClassMap } from "./fields/Typography"
import { generateTailwindClasses } from "./fields/unit"
import { cn } from "@/utils"

export interface LinkProps {
    text: string
    href: string
    typography: TypographyProps
}

export const Link: ComponentConfig<LinkProps> = {
    fields: {
        text: { type: "text" },
        href: { type: "text" },
        typography: Typography,
    },
    defaultProps: {
        text: "Click here",
        href: "#",
        typography: {},
    },
    render: ({ text, href, typography }) => {
        const classes = cn(generateTailwindClasses(typography, typographyClassMap))
        return (
            <EmailLink href={href} className={classes}>
                {text}
            </EmailLink>
        )
    },
}
