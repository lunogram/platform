type Listener = () => void;

class CodeStore {
  private code = "";
  private listeners = new Set<Listener>();

  getCode() {
    return this.code;
  }

  setCode(next: string) {
    if (next === this.code) return;
    this.code = next;
    this.listeners.forEach(l => l());
  }

  subscribe(listener: Listener) {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  }
}

export const codeStore = new CodeStore();
