import {
  dracula,
  CodeBlock as EmailCodeBlock,
  nord,
  type PrismLanguage,
  type Theme,
} from "@react-email/components";
import type { ComponentConfig } from "@puckeditor/core";
import { Spacing, type SpacingProps, spacingClassMap } from "./fields/Spacing";
import { generateTailwindClasses } from "./fields/unit";
import { cn } from "@/utils";

export interface CodeBlockProps {
  code: string;
  language: PrismLanguage;
  lineNumbers?: boolean;
  theme?: string;
  spacing: SpacingProps;
}

const themeMap: Record<string, Theme> = {
  dracula: dracula,
  nord: nord,
};

export const CodeBlock: ComponentConfig<CodeBlockProps> = {
  fields: {
    code: { type: "textarea" },
    language: {
      type: "select",
      options: [
        { label: "JavaScript", value: "javascript" },
        { label: "TypeScript", value: "typescript" },
      ],
    },
    lineNumbers: {
      type: "select",
      options: [
        { label: "True", value: "true" },
        { label: "False", value: "false" },
      ],
    },
    theme: {
      type: "select",
      options: [
        { label: "Dracula", value: "dracula" },
        { label: "Nord", value: "nord" },
      ],
    },
    spacing: Spacing,
  },
  defaultProps: {
    code: 'console.log("Hello World");',
    language: "javascript",
    theme: "dracula",
    spacing: {},
  },
  render: ({ code, language, theme, spacing, lineNumbers }) => {
    const classes = cn(generateTailwindClasses(spacing, spacingClassMap));
    return (
      <div className={classes}>
        <EmailCodeBlock
          code={code}
          language={language}
          lineNumbers={lineNumbers}
          theme={themeMap[theme!] || dracula}
        />
      </div>
    );
  },
};
