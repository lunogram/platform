import { transform } from "sucrase";
import React from "react";
import { jsx, jsxs, Fragment } from "react/jsx-runtime";
import { render } from "@react-email/render";
import * as ReactEmailComponents from "@react-email/components";

/**
 * The scope available to user templates at runtime.
 * Includes JSX runtime functions and all @react-email/components exports.
 */
const REACT_EMAIL_SCOPE: Record<string, unknown> = {
  _jsx: jsx,
  _jsxs: jsxs,
  _Fragment: Fragment,
  React,
  ...ReactEmailComponents,
};

/**
 * Transpile and compile a React Email JSX source string into executable JS.
 *
 * The compiled output is a self-contained function body that:
 * 1. Receives the react-email scope + any tailwind config bindings as arguments
 * 2. Returns the default-exported React component
 *
 * The compiled string is stored and reused at render time with different props.
 */
export function compile(source: string): string {
  const transformed = transform(source, {
    transforms: ["jsx", "typescript"],
    jsxRuntime: "automatic",
    jsxImportSource: "react",
    production: true,
  });

  let code = transformed.code;

  // Detect tailwind.config imports before stripping
  const tailwindConfigBindings: string[] = [];
  const tailwindImportRe =
    /^import\s+(\w+)\s+from\s+['"].*tailwind\.config(?:\.ts|\.js|\.mjs)?['"];?\s*$/gm;
  let match: RegExpExecArray | null;
  while ((match = tailwindImportRe.exec(code)) !== null) {
    tailwindConfigBindings.push(match[1]);
  }

  // Strip import statements
  code = code.replace(
    /^import\s+[\s\S]*?from\s+['"].*?['"];?\s*$/gm,
    ""
  );
  code = code.replace(/^import\s+['"].*?['"];?\s*$/gm, "");

  // Replace default export with variable assignment
  code = code.replace(/export\s+default\s+/, "var __Component__ = ");

  // Remove named exports
  code = code.replace(
    /export\s+(?:const|let|var|function|class)\s+/g,
    "var "
  );

  // Return the compiled code with metadata about tailwind config bindings
  return JSON.stringify({
    code,
    tailwindConfigBindings,
  });
}

/**
 * Render a pre-compiled template with the given props.
 *
 * The compiledBundle is the JSON string returned by compile().
 * Props are passed to the component as its single argument.
 */
export async function renderTemplate(
  compiledBundle: string,
  props: Record<string, unknown>
): Promise<{ html: string; plainText: string }> {
  const { code, tailwindConfigBindings } = JSON.parse(compiledBundle);

  const scope: Record<string, unknown> = {
    ...REACT_EMAIL_SCOPE,
  };

  // Inject default tailwind config for any detected bindings
  if (tailwindConfigBindings.length > 0) {
    const defaultConfig = {
      presets: [ReactEmailComponents.pixelBasedPreset],
    };
    for (const binding of tailwindConfigBindings) {
      scope[binding] = defaultConfig;
    }
  }

  const scopeKeys = Object.keys(scope);
  const scopeValues = scopeKeys.map((k) => scope[k]);

  // Execute the compiled code to get the component constructor
  const fn = new Function(
    ...scopeKeys,
    `${code}\nreturn typeof __Component__ !== 'undefined' ? __Component__ : null;`
  );
  const Component = fn(...scopeValues) as React.ComponentType<
    Record<string, unknown>
  > | null;

  if (!Component) {
    throw new Error(
      "No default export found in compiled template."
    );
  }

  // Render with props
  const element = React.createElement(Component, props);
  const html = await render(element);
  const plainText = await render(element, { plainText: true });

  return { html, plainText };
}
