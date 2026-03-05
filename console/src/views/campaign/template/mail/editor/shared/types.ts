// shared/types.ts

/* ============================
   EDITOR
   ============================ */

/**
 * Raw editor value.
 * Changes constantly.
 * Not observable.
 */
export type EditorCode = string

/**
 * Editor change callback.
 * Fired by Monaco / CodeMirror.
 */
export type EditorChangeListener = (code: EditorCode | undefined) => void

/* ============================
   STORE / PUBSUB
   ============================ */

/**
 * Function used to unsubscribe.
 */
export type Unsubscribe = () => void

/**
 * Generic listener signature.
 */
export type Listener<T> = (value: T) => void

/* ============================
   DEBOUNCE / SCHEDULING
   ============================ */

/**
 * Debounced executor.
 * Used to control when compilation happens.
 */
export type DebouncedFn<T extends Array<string | undefined>> = (...args: T) => void
