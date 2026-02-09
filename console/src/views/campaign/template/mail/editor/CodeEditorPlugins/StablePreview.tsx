import { useMemo } from "react";
import { useSyncExternalStore } from "react";
import { codeStore } from "./codeStore";
import { compileCode } from "./compileCode";

export function StablePreview() {
  const code = useSyncExternalStore(
    codeStore.subscribe.bind(codeStore),
    () => codeStore.getCode()
  );

  const result = useMemo(() => compileCode(code), [code]);

  if (result.error) {
    return <pre style={{ color: "red" }}>{result.error.message}</pre>;
  }

  if (!result.component) {
    return <div>No preview</div>;
  }

  const Comp = result.component;
  return <Comp />;
}
