import type { ComponentConfig, Slot } from '@puckeditor/core'; 
import { Container as EmailContainer, Section as EmailSection } from '@react-email/components';
import { Layout, type LayoutProps, layoutClassMap } from './fields/Layout';
import { cn } from '@/utils';
import { Spacing, type SpacingProps, spacingClassMap } from './fields/Spacing';
import { Typography, type TypographyProps, typographyClassMap } from './fields/Typography';
import { Decoration, type DecorationProps, decorationClassMap } from './fields/Decoration';
import { generateTailwindClasses } from './fields/unit';

export interface ContainerProps {
    content: Slot; 
    layout: LayoutProps;
    spacing: SpacingProps;
    typography: TypographyProps;
    decoration: DecorationProps;
};

export const Container: ComponentConfig<ContainerProps> = {
    fields: {
        content: {
            type: "slot",
        },
        layout: Layout,
        typography: Typography,
        spacing: Spacing,
        decoration: Decoration,
    },
    defaultProps: {
        content: [],
        layout: {},
        typography: {},
        spacing: {},
        decoration: {},
    },
    render: ({ content: Content, layout, spacing, typography, decoration }) => {
        const sectionClasses = cn(
            generateTailwindClasses(layout, layoutClassMap),
            generateTailwindClasses(spacing, spacingClassMap),
            generateTailwindClasses(typography, typographyClassMap),
            generateTailwindClasses(decoration, decorationClassMap),
        )

        return (
            <EmailSection className={sectionClasses}>
                <EmailContainer style={{ margin: '0 auto' }}>
                    <Content />
                </EmailContainer>
            </EmailSection>
        );
    },
}