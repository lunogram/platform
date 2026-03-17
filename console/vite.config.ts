import { defineConfig, loadEnv, type Plugin } from "vite"
import react from "@vitejs/plugin-react"
import tailwindcss from "@tailwindcss/vite"
import path from "path"
import fs from "fs"

const ENTERPRISE_PKG_PREFIX = "@lunogram-enterprise/"

/**
 * Vite plugin that stubs out `@lunogram-enterprise/*` imports when building
 * the open-source edition (`VITE_ENTERPRISE !== "true"`).
 *
 * The plugin scans all source files for imports from enterprise packages,
 * collects every named export that is referenced, and generates virtual
 * stub modules that re-export matching names — each pointing at a single
 * Proxy-backed no-op so that destructuring, JSX rendering, and hook calls
 * all silently return safe inert values.
 *
 * Because every call-site is behind an `if (isEnterprise)` / `__ENTERPRISE__`
 * guard, Vite's dead-code elimination removes all references at build time,
 * but Rollup no longer errors on unresolvable imports during bundling.
 *
 * In enterprise builds the plugin is not loaded, so the real workspace
 * packages are resolved normally.
 */
function enterpriseStubPlugin(): Plugin {
    const VIRTUAL_PREFIX = "\0enterprise-stub:"

    // Collect named imports per enterprise module specifier.
    // Key = full import specifier (e.g. "@lunogram-enterprise/ai-builder")
    // Value = Set of locally-imported names
    const namedImportsMap = new Map<string, Set<string>>()

    /**
     * Scan a piece of source code for any `import { … } from "@lunogram-enterprise/…"`
     * or `import X from "@lunogram-enterprise/…"` statements and record the names.
     */
    function collectEnterpriseImports(code: string) {
        // Match:  import { A, B as C } from "@lunogram-enterprise/…"
        //         import type { A } from "@lunogram-enterprise/…"  (also matched, harmless)
        const namedRe =
            /import\s+(?:type\s+)?{([^}]+)}\s+from\s+["'](@lunogram-enterprise\/[^"']+)["']/g
        let m: RegExpExecArray | null
        while ((m = namedRe.exec(code)) !== null) {
            const names = m[1]
            const specifier = m[2]
            if (!namedImportsMap.has(specifier)) {
                namedImportsMap.set(specifier, new Set())
            }
            const set = namedImportsMap.get(specifier)!
            for (const part of names.split(",")) {
                // "  BuilderProvider  " or "  useX as aliasX  "
                const trimmed = part.trim()
                if (!trimmed) continue
                // For `A as B`, we need A (the exported name from the module)
                const exportedName = trimmed.split(/\s+as\s+/)[0].trim()
                if (exportedName) set.add(exportedName)
            }
        }

        // Match:  import Foo from "@lunogram-enterprise/…"
        const defaultRe = /import\s+(\w+)\s+from\s+["'](@lunogram-enterprise\/[^"']+)["']/g
        while ((m = defaultRe.exec(code)) !== null) {
            const specifier = m[2]
            if (!namedImportsMap.has(specifier)) {
                namedImportsMap.set(specifier, new Set())
            }
            // default import is handled by `export default` in the stub,
            // no need to add to the named set
        }
    }

    /**
     * Recursively scan the src directory to pre-collect all enterprise imports
     * before Rollup starts resolving.
     */
    function prescanSourceDir(dir: string) {
        if (!fs.existsSync(dir)) return
        for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
            const full = path.join(dir, entry.name)
            if (entry.isDirectory()) {
                prescanSourceDir(full)
            } else if (/\.(tsx?|jsx?|mjs|mts)$/.test(entry.name)) {
                const code = fs.readFileSync(full, "utf-8")
                collectEnterpriseImports(code)
            }
        }
    }

    return {
        name: "enterprise-stub",
        enforce: "pre",

        buildStart() {
            // Pre-scan the source tree so we know all named imports upfront
            const srcDir = path.resolve(process.cwd(), "src")
            prescanSourceDir(srcDir)
        },

        resolveId(source) {
            if (source.startsWith(ENTERPRISE_PKG_PREFIX)) {
                return VIRTUAL_PREFIX + source
            }
        },

        transform(code, id) {
            // Also collect imports at transform time to catch anything the
            // pre-scan might have missed (e.g. generated files).
            if (code.includes(ENTERPRISE_PKG_PREFIX)) {
                collectEnterpriseImports(code)
            }
            return null
        },

        load(id) {
            if (!id.startsWith(VIRTUAL_PREFIX)) return null

            const specifier = id.slice(VIRTUAL_PREFIX.length)
            const names = namedImportsMap.get(specifier) ?? new Set<string>()

            // Build the stub module.
            //
            // `stubProxy` is a Proxy-backed no-op function:
            //   - Calling it returns itself      → hooks like useX() work
            //   - Property access returns itself  → destructuring works
            //   - Used as a React component       → renders nothing (returns proxy,
            //     and the code path is dead anyway so React never actually sees it)
            const namedExports = [...names].map((n) => `export const ${n} = stubProxy;`).join("\n")

            return `
const noop = () => stubProxy;
const stubProxy = new Proxy(noop, {
    get(_target, prop) {
        if (prop === Symbol.toPrimitive) return () => "";
        if (prop === "$$typeof") return undefined;
        if (prop === "__esModule") return true;
        if (prop === "then") return undefined;
        return stubProxy;
    },
    apply() {
        return stubProxy;
    },
});
export default stubProxy;
${namedExports}
`
        },
    }
}

export default defineConfig(({ mode }) => {
    const env = loadEnv(mode, process.cwd(), "")

    const isEnterprise = env.VITE_ENTERPRISE === "true"

    const favicon = mode === "development" ? "/favicon-dev.png" : "/favicon.ico"

    const faviconPlugin: Plugin = {
        name: "favicon",
        transformIndexHtml() {
            return [
                {
                    tag: "link",
                    attrs: { rel: "icon", href: favicon, type: "image/png" },
                    injectTo: "head" as const,
                },
            ]
        },
    }

    return {
        base: "/",
        define: {
            __ENTERPRISE__: JSON.stringify(isEnterprise),
        },
        plugins: [
            // Stub enterprise packages in OSS builds so Rollup doesn't choke
            // on unresolvable imports.
            ...(!isEnterprise ? [enterpriseStubPlugin()] : []),
            react(),
            tailwindcss(),
            faviconPlugin,
        ],
        server: {
            proxy: {
                "/api": {
                    target: env.VITE_PROXY_URL,
                    changeOrigin: true,
                },
                "/backoffice": {
                    target: env.VITE_BACKOFFICE_URL || "http://localhost:8081",
                    changeOrigin: true,
                    rewrite: (path) => path.replace(/^\/backoffice/, ""),
                },
                "/courier": {
                    target: env.VITE_COURIER_URL || "http://localhost:8082",
                    changeOrigin: true,
                    rewrite: (path) => path.replace(/^\/courier/, ""),
                },
                "/unsubscribe": {
                    target: env.VITE_PROXY_URL,
                    changeOrigin: true,
                    rewrite: (path) => path.replace(/^\/unsubscribe/, "/api/unsubscribe"),
                },
            },
        },
        test: {
            globals: true,
            environment: "jsdom",
            setupFiles: "./src/setupTests.ts",
            css: true,
            reporters: ["verbose"],
            coverage: {
                reporter: ["text", "json", "html"],
                include: ["src/**/*"],
                exclude: [],
            },
        },
        worker: {
            format: "es",
        },
        resolve: {
            alias: {
                "@": path.resolve(__dirname, "./src"),
            },
        },
    }
})
