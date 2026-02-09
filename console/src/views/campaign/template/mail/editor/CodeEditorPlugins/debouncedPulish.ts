import { codeStore } from "./codeStore";

let timeout: ReturnType<typeof setTimeout> | null = null;

export function debouncedPublish(code: string, delay = 400) {
  if (timeout) {
    clearTimeout(timeout);
  }

  timeout = setTimeout(() => {
    codeStore.setCode(code);
  }, delay);
}
