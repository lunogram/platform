/**
 * Stub for `pusher-js`, aliased in vite.config.ts.
 *
 * `@templatical/editor` dynamically imports pusher-js from its cloud chunk to
 * power realtime collaboration in `initCloud()`. The console only ever calls
 * the self-hosted `init()`, so that chunk is never loaded — but Rollup still
 * has to resolve the import at build time, and pusher-js is not among the
 * package's declared dependencies.
 *
 * Aliasing it here keeps a transport library we never use out of the bundle.
 * If the cloud editor is ever adopted, delete this stub and add the real
 * dependency instead.
 */
export default class PusherStub {
    constructor() {
        throw new Error(
            "pusher-js is stubbed: the Templatical cloud editor is not enabled in this build.",
        )
    }
}
