import { Section as EmailSection } from "@react-email/components";
import type { ComponentConfig, Slot } from "@puckeditor/core";
import { Layout, type LayoutProps, layoutClassMap } from "./fields/Layout";
import { Spacing, type SpacingProps, spacingClassMap } from "./fields/Spacing";
import {
  Decoration,
  type DecorationProps,
  decorationClassMap,
} from "./fields/Decoration";
import { generateTailwindClasses } from "./fields/unit";
import { cn } from "@/utils";

export interface SectionProps {
  content: Slot;
  layout: LayoutProps;
  spacing: SpacingProps;
  decoration: DecorationProps;
}

export const Section: ComponentConfig<SectionProps> = {
  fields: {
    content: { type: "slot" },
    layout: Layout,
    spacing: Spacing,
    decoration: Decoration,
  },
  defaultProps: {
    content: [],
    layout: {},
    spacing: {},
    decoration: {},
  },
  render: ({ content: Content, layout, spacing, decoration }) => {
    const classes = cn(
      generateTailwindClasses(layout, layoutClassMap),
      generateTailwindClasses(spacing, spacingClassMap),
      generateTailwindClasses(decoration, decorationClassMap),
    );
    return (
      <EmailSection className={classes}>
        <Content />
      </EmailSection>
    );
  },
};
