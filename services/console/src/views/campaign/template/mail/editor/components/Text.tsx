import type { ComponentConfig, Slot } from '@measured/puck';
import { Layout, type LayoutProps, layoutClassMap } from './fields/Layout';
import { cn } from '@/utils';
import { Spacing, type SpacingProps, spacingClassMap } from './fields/Spacing';
import { Typography, type TypographyProps, typographyClassMap } from './fields/Typography';
import { Decoration, type DecorationProps, decorationClassMap } from './fields/Decoration';
import { generateTailwindClasses } from './fields/unit';

export interface TextProps {
    content: Slot;
    layout: LayoutProps;
    spacing: SpacingProps;
    typography: TypographyProps;
    decoration: DecorationProps;
};

export const Text: ComponentConfig<TextProps> = {
    fields: {
        content: {
            type: "slot",
        },
        layout: Layout,
        spacing: Spacing,
        typography: Typography,
        decoration: Decoration,
    },
    defaultProps: {
        content: [
            {
                type: "TextSection",
                props: {},
            }
        ],
        layout: {},
        spacing: {},
        typography: {
            xl: {
                fontSize: '16',
                fontWeight: '600',
                color: '#000000',
            }
        },
        decoration: {},
    },
    render: ({ content: Content, layout, spacing, typography, decoration }) => {
        const classes = cn(
            "puck-text-component",
            generateTailwindClasses(layout, layoutClassMap),
            generateTailwindClasses(spacing, spacingClassMap),
            generateTailwindClasses(typography, typographyClassMap),
            generateTailwindClasses(decoration, decorationClassMap),
        )

        return (
            <Content as="p" className={classes} />
        );
    },
}
