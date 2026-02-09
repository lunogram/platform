// preview/StablePreview.tsx
import React, { useMemo } from "react";
import { useSyncExternalStore } from "react";
import { Render } from "@puckeditor/core";
import { codeStore } from "./codeStore";
import { compileCode } from "./compileCode";

export function StablePreview(props: any) {
  const code = useSyncExternalStore(
    codeStore.subscribe.bind(codeStore),
    () => codeStore.getCode()
  );

  const result = useMemo(() => compileCode(code), [code]);

  if (result.component) {
    const Comp = result.component;
    return <Comp />;
  }

  return <Render {...props} />;
}
