import { Row as EmailRow } from "@react-email/components";
import type { ComponentConfig, Slot } from "@puckeditor/core";
import { Spacing, type SpacingProps, spacingClassMap } from "./fields/Spacing";
import { generateTailwindClasses } from "./fields/unit";
import { cn } from "@/utils";

export interface RowProps {
  content: Slot;
  spacing: SpacingProps;
}

export const Row: ComponentConfig<RowProps> = {
  fields: {
    content: { type: "slot" },
    spacing: Spacing,
  },
  defaultProps: {
    content: [],
    spacing: {},
  },
  render: ({ content: Content, spacing }) => {
    const classes = cn(generateTailwindClasses(spacing, spacingClassMap));
    return (
      <EmailRow className={classes}>
        <Content />
      </EmailRow>
    );
  },
};