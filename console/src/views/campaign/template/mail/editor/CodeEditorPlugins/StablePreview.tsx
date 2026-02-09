// preview/StablePreview.tsx
import { useMemo } from "react";
import { useSyncExternalStore } from "react";
import { Render as DefaultRender } from "@puckeditor/core";

import { codeStore } from "./codeStore";
import { compileCode } from "./compileCode";

export function StablePreview(puckProps: any) {
  const code = useSyncExternalStore(codeStore.subscribe.bind(codeStore), () =>
    codeStore.getCode(),
  );

  const result = useMemo(() => compileCode(code), [code]);

  if (result.component) {
    const Comp = result.component;
    return <Comp />;
  }

  return <DefaultRender {...puckProps} />;
}
