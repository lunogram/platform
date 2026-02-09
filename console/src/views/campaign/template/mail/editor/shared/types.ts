// shared/types.ts

/* ============================
   EDITOR
   ============================ */

/**
 * Raw editor value.
 * Changes constantly.
 * Not observable.
 */
export type EditorCode = string;

/**
 * Singleton ref shape.
 * React must NEVER depend on this directly.
 */
export interface EditorRef {
  current: EditorCode;
}

/**
 * Editor change callback.
 * Fired by Monaco / CodeMirror.
 */
export type EditorChangeListener = (code: EditorCode) => void;


/* ============================
   STORE / PUBSUB
   ============================ */

/**
 * Function used to unsubscribe.
 */
export type Unsubscribe = () => void;

/**
 * Generic listener signature.
 */
export type Listener<T> = (value: T) => void;

/**
 * External store interface.
 * Manual pub/sub, no React.
 */
export interface ExternalStore<T> {
  get(): T;
  set(value: T): void;
  subscribe(listener: Listener<T>): Unsubscribe;
}


/* ============================
   COMPILATION
   ============================ */

/**
 * Result of compiling editor code.
 * NEVER raw JSX directly.
 */
export interface CompileResult {
  component: React.ComponentType<any> | null;
  error: CompileError | null;
}

/**
 * Compiler error shape.
 * Keep it serializable.
 */
export interface CompileError {
  message: string;
  line?: number;
  column?: number;
  stack?: string;
}

/**
 * Compiler function signature.
 * Must be pure-ish.
 */
export type Compiler = (code: EditorCode) => CompileResult;


/* ============================
   PREVIEW
   ============================ */

/**
 * Increment-only version.
 * Used to force safe re-renders.
 */
export type Version = number;

/**
 * What the preview subscribes to.
 */
export interface PreviewState {
  version: Version;
  result: CompileResult;
}


/* ============================
   DEBOUNCE / SCHEDULING
   ============================ */

/**
 * Debounced executor.
 * Used to control when compilation happens.
 */
export type DebouncedFn<T extends any[]> = (...args: T) => void;


/* ============================
   PUCK
   ============================ */

/**
 * Props passed to the stable preview component.
 * MUST stay referentially stable.
 */
export interface StablePreviewProps {
  version: Version;
  component: React.ComponentType<any> | null;
  error: CompileError | null;
}
