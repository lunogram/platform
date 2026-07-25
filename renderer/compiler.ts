import { transform } from "sucrase";
import React from "react";
import { jsx, jsxs, Fragment } from "react/jsx-runtime";
import { render } from "@react-email/render";
import * as ReactEmailComponents from "@react-email/components";
import {
  isTemplaticalDocument,
  renderTemplaticalDocument,
} from "./templatical.ts";

/**
 * Marks a bundle produced from a Templatical document rather than React Email
 * JSX. Bundles without a `kind` predate this branch and are React Email, so
 * existing templates keep working untouched.
 */
const TEMPLATICAL_KIND = "templatical";

/** Parse `source` as a Templatical document, or return null if it is JSX. */
function asTemplaticalDocument(source: string) {
  let parsed: unknown;
  try {
    parsed = JSON.parse(source);
  } catch {
    return null;
  }
  return isTemplaticalDocument(parsed) ? parsed : null;
}

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
 * Compile a template source into a bundle that render time can reuse.
 *
 * Two kinds of source are accepted:
 *
 * - A **Templatical document** (JSON). Rendering it depends only on the
 *   document — merge tags resolve downstream via Liquid, not from props — so
 *   the final HTML is produced here, once, and every recipient reuses it.
 * - **React Email JSX**, which is transpiled to a self-contained function body
 *   that receives the react-email scope and returns the default export. It is
 *   executed per render because its output depends on props.
 */
export async function compile(source: string): Promise<string> {
  const doc = asTemplaticalDocument(source);
  if (doc) {
    const { html, plainText } = await renderTemplaticalDocument(doc);
    return JSON.stringify({ kind: TEMPLATICAL_KIND, html, plainText });
  }

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
  const bundle = JSON.parse(compiledBundle);

  // Templatical bundles were fully rendered at compile time — there is nothing
  // props could change, so hand back the stored output.
  if (bundle.kind === TEMPLATICAL_KIND) {
    return { html: bundle.html, plainText: bundle.plainText };
  }

  const { code, tailwindConfigBindings } = bundle;

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
