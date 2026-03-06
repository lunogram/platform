import {
    dracula,
    CodeBlock as EmailCodeBlock,
    nord,
    vscDarkPlus,
    type PrismLanguage,
    type Theme,
} from "@react-email/components"
import type { ComponentConfig } from "@puckeditor/core"
import { Spacing, type SpacingProps, spacingClassMap } from "./fields/Spacing"
import { generateTailwindClasses } from "./fields/unit"
import { cn } from "@/utils"

export interface CodeBlockProps {
    code: string
    language: PrismLanguage
    lineNumbers?: boolean
    theme?: keyof Theme
    spacing: SpacingProps
}

const themeMap: Record<keyof Theme, Theme> = {
    vscDarkPlus: vscDarkPlus,
    dracula: dracula,
    nord: nord,
}

const themeOptions: { label: string; value: keyof Theme }[] = [
    { label: "Dracula", value: "dracula" },
    { label: "Nord", value: "nord" },
    { label: "VS Code Dark+", value: "vscDarkPlus" },
]

const languages: { label: string; value: PrismLanguage }[] = [
    { label: "Bash", value: "bash" },
    { label: "C#", value: "csharp" },
    { label: "C++", value: "cpp" },
    { label: "CSS", value: "css" },
    { label: "Docker", value: "docker" },
    { label: "Go", value: "go" },
    { label: "GraphQL", value: "graphql" },
    { label: "HTML/XML", value: "markup" },
    { label: "Java", value: "java" },
    { label: "JavaScript", value: "javascript" },
    { label: "JSON", value: "json" },
    { label: "JSX", value: "jsx" },
    { label: "Markdown", value: "markdown" },
    { label: "PHP", value: "php" },
    { label: "Python", value: "python" },
    { label: "Rust", value: "rust" },
    { label: "SQL", value: "sql" },
    { label: "TSX", value: "tsx" },
    { label: "TypeScript", value: "typescript" },
    { label: "YAML", value: "yaml" },
]

export const CodeBlock: ComponentConfig<CodeBlockProps> = {
    fields: {
        code: { type: "textarea" },
        language: {
            type: "select",
            options: languages,
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
            options: themeOptions as { label: string; value: string }[],
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
        const classes = cn(generateTailwindClasses(spacing, spacingClassMap))
        return (
            <div className={classes}>
                <EmailCodeBlock
                    code={code}
                    language={language}
                    lineNumbers={lineNumbers}
                    theme={themeMap[theme!] || vscDarkPlus}
                />
            </div>
        )
    },
}
