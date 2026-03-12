/**
 * Prevent Prism.js (bundled inside @react-email/components) from registering
 * its own `message` event handler when loaded inside a Web Worker.
 *
 * Prism's handler calls `JSON.parse(evt.data)` expecting a string, but our
 * compile worker posts structured objects via `postMessage()`. The uncaught
 * `SyntaxError` crashes the worker and prevents email preview rendering.
 *
 * This module MUST be imported before `@react-email/components` so the flag
 * is set before Prism's IIFE runs.
 */

// eslint-disable-next-line @typescript-eslint/no-explicit-any
;(self as any).Prism = { disableWorkerMessageHandler: true }
