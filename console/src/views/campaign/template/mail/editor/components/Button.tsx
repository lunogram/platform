import type { ComponentConfig } from '@measured/puck';
import { Button as EmailButton } from '@react-email/components';
import { Layout, type LayoutProps, layoutClassMap } from './fields/Layout';
import { cn } from '@/utils';
import { Spacing, type SpacingProps, spacingClassMap } from './fields/Spacing';
import { Typography, type TypographyProps, typographyClassMap } from './fields/Typography';
import { Decoration, type DecorationProps, decorationClassMap } from './fields/Decoration';
import { generateTailwindClasses } from './fields/unit';

export interface ButtonProps {
    value: string;
    href: string;
    layout: LayoutProps;
    spacing: SpacingProps;
    typography: TypographyProps;
    decoration: DecorationProps;
};

export const Button: ComponentConfig<ButtonProps> = {
    fields: {
        value: {
            type: "text",
        },
        href: {
            type: "text",
        },
        layout: Layout,
        typography: Typography,
        spacing: Spacing,
        decoration: Decoration,
    },
    defaultProps: {
        value: "Next",
        href: "#",
        layout: {
            xl: {
                width: '100%',
            }
        },
        typography: {
            xl: {
                textAlign: 'center',
                fontSize: '16',
                fontWeight: 'semibold',
                color: '#ffffff',
            }
        },
        spacing: {},
        decoration: {
            xl: {
                borderTopLeftRadius: '8',
                borderTopRightRadius: '8',
                borderBottomLeftRadius: '8',
                borderBottomRightRadius: '8',
                backgroundColor: '#4f46e5',
            }
        },
    },
    render: ({ value, href, layout, spacing, typography, decoration }) => {
        const classes = cn(
            "box-border",
            generateTailwindClasses(layout, layoutClassMap),
            generateTailwindClasses(spacing, spacingClassMap),
            generateTailwindClasses(typography, typographyClassMap),
            generateTailwindClasses(decoration, decorationClassMap),
        )

        return (
            <EmailButton className={classes} href={href}>
                {value}
            </EmailButton>
        );
    },
}
