import type { OnMount } from "@monaco-editor/react"

const REACT_TYPE_DECLARATIONS = `declare namespace React {
    interface ReactElement<P = any> {
        type: any;
        props: P;
        key: string | null;
    }
    type ReactNode = ReactElement | string | number | boolean | null | undefined | ReactNode[];
    type FC<P = {}> = (props: P) => ReactElement | null;
    type CSSProperties = { [key: string]: string | number | undefined };
    interface HTMLAttributes<T> {
        style?: CSSProperties;
        className?: string;
        children?: ReactNode;
        id?: string;
    }
    function createElement(type: any, props?: any, ...children: any[]): ReactElement;
    const Fragment: symbol;
}

declare global {
    namespace JSX {
        interface Element extends React.ReactElement<any> {}
        interface IntrinsicElements {
            [elemName: string]: any;
        }
    }
}

declare module "react" {
    export = React;
}

declare module "react/jsx-runtime" {
    export function jsx(type: any, props: any, key?: any): JSX.Element;
    export function jsxs(type: any, props: any, key?: any): JSX.Element;
    export const Fragment: symbol;
}`

const REACT_EMAIL_TYPE_DECLARATIONS = `declare module "@react-email/components" {
    interface BaseProps {
        style?: React.CSSProperties;
        className?: string;
        children?: React.ReactNode;
    }

    interface TailwindConfig {
        presets?: any[];
        theme?: {
            extend?: Record<string, any>;
            [key: string]: any;
        };
        [key: string]: any;
    }

    export const Html: React.FC<BaseProps>;
    export const Head: React.FC<{ children?: React.ReactNode }>;
    export const Body: React.FC<BaseProps>;
    export const Container: React.FC<BaseProps>;
    export const Section: React.FC<BaseProps>;
    export const Row: React.FC<BaseProps>;
    export const Column: React.FC<BaseProps>;
    export const Text: React.FC<BaseProps>;
    export const Link: React.FC<BaseProps & { href?: string }>;
    export const Button: React.FC<BaseProps & { href?: string }>;
    export const Img: React.FC<{ src?: string; alt?: string; width?: number | string; height?: number | string; style?: React.CSSProperties; className?: string }>;
    export const Hr: React.FC<{ style?: React.CSSProperties; className?: string }>;
    export const Preview: React.FC<{ children?: React.ReactNode }>;
    export const Heading: React.FC<BaseProps & { as?: "h1" | "h2" | "h3" | "h4" | "h5" | "h6" }>;
    export const Font: React.FC<{ fontFamily: string; fallbackFontFamily?: string; webFont?: { url: string; format: string }; fontStyle?: string; fontWeight?: number | string }>;
    export const Tailwind: React.FC<{ config?: TailwindConfig; children?: React.ReactNode }>;
    export const Markdown: React.FC<{ markdownContainerStyles?: React.CSSProperties; markdownCustomStyles?: Record<string, React.CSSProperties>; children: string }>;
    export const CodeBlock: React.FC<{ code: string; language?: string; theme?: Record<string, unknown>; style?: React.CSSProperties; className?: string }>;
    export const CodeInline: React.FC<BaseProps>;

    export const pixelBasedPreset: any;
}`

const TAILWIND_CONFIG_DECLARATION = `
    import type { TailwindConfig } from "@react-email/components";
    declare const config: TailwindConfig;
    export default config;
`

const TAILWIND_CONFIG_PATHS = [
    "file:///tailwind.config.ts",
    "file:///tailwind.config.js",
    "file:///tailwind.config.mjs",
]

/**
 * Configure the Monaco editor for JSX/TSX editing with full IntelliSense
 * support for React, React Email components, and EmailProps.
 *
 * This sets up:
 * - TypeScript compiler options for automatic JSX transform (React 17+)
 * - React and JSX type declarations
 * - React Email component type declarations
 * - EmailProps interface for template props IntelliSense
 * - Virtual tailwind.config module types
 *
 * @param editor - The Monaco editor instance
 * @param monaco - The Monaco namespace
 * @param propsTypeDeclarations - Generated TS declarations for EmailProps interface
 */
export function configureMonaco(
    editor: Parameters<OnMount>[0],
    monaco: Parameters<OnMount>[1],
    propsTypeDeclarations: string,
): void {
    // Configure JSX/TSX support with automatic JSX transform (React 17+)
    // No need for `import React` in user code
    monaco.languages.typescript.typescriptDefaults.setCompilerOptions({
        target: monaco.languages.typescript.ScriptTarget.Latest,
        allowNonTsExtensions: true,
        moduleResolution: monaco.languages.typescript.ModuleResolutionKind.NodeJs,
        module: monaco.languages.typescript.ModuleKind.ESNext,
        jsx: monaco.languages.typescript.JsxEmit.ReactJSX,
        allowJs: true,
        strict: false,
        noImplicitAny: false,
        skipLibCheck: true,
    })

    // Add comprehensive React and JSX types
    monaco.languages.typescript.typescriptDefaults.addExtraLib(
        REACT_TYPE_DECLARATIONS,
        "file:///node_modules/@types/react/index.d.ts",
    )

    // Add react-email component types (all components support className for Tailwind CSS)
    monaco.languages.typescript.typescriptDefaults.addExtraLib(
        REACT_EMAIL_TYPE_DECLARATIONS,
        "file:///node_modules/@react-email/components/index.d.ts",
    )

    // Set model language to typescript for JSX support
    const model = editor.getModel()
    if (model) {
        monaco.editor.setModelLanguage(model, "typescript")
    }

    // Add EmailProps interface as a global type declaration so the user
    // can write `export default function Email(props: EmailProps) { ... }`
    // and get full IntelliSense on `props.user.first_name` etc.
    monaco.languages.typescript.typescriptDefaults.addExtraLib(
        propsTypeDeclarations,
        "file:///email-props.d.ts",
    )

    // Add tailwind.config virtual module types.
    // Supports common import paths: '../tailwind.config', './tailwind.config',
    // and variants with .ts/.js/.mjs extensions.
    for (const p of TAILWIND_CONFIG_PATHS) {
        monaco.languages.typescript.typescriptDefaults.addExtraLib(TAILWIND_CONFIG_DECLARATION, p)
    }
}

/**
 * Update the EmailProps type declarations in an already-configured
 * Monaco instance. Call this when variable groups change after initial mount.
 *
 * @param monaco - The Monaco namespace (from the editor's onMount or stored ref)
 * @param propsTypeDeclarations - Updated TS declarations for the EmailProps interface
 */
export function updatePropsTypeDeclarations(
    monaco: Parameters<OnMount>[1],
    propsTypeDeclarations: string,
): void {
    monaco.languages.typescript.typescriptDefaults.addExtraLib(
        propsTypeDeclarations,
        "file:///email-props.d.ts",
    )
}
