import * as React from "react";

export type CompileResult =
  | { component: React.ComponentType<unknown>; error: null }
  | { component: null; error: Error | null };

export function compileCode(code: string): CompileResult {
  try {
    if (!code.trim()) {
      return { component: null, error: null };
    }

    const wrapped = `
      ${code}
      return Preview;
    `;

    const factory = new Function("React", wrapped);
    const Comp = factory(React);

    if (typeof Comp !== "function") {
      throw new Error("Preview must be a React component");
    }

    return { component: Comp, error: null };
  } catch (err) {
    return {
      component: null,
      error: err instanceof Error ? err : new Error("Unknown error"),
    };
  }
}
