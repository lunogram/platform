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
 * In enterprise builds the resolve plugin below handles resolution instead.
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

        transform(code) {
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

/**
 * Vite plugin that resolves `@lunogram-enterprise/*` imports directly to
 * source files in the `packages/` directory for enterprise builds.
 *
 * This avoids the need for pnpm to link the enterprise workspace packages
 * into `node_modules`, which is fragile across the two different build
 * contexts (root workspace for enterprise vs standalone for OSS).
 *
 * The plugin reads each package's `package.json` exports map to resolve
 * subpath exports (e.g. `@lunogram-enterprise/oapi-client/courier` →
 * `packages/oapi-client/src/courier-client.ts`).
 */
function enterpriseResolvePlugin(): Plugin {
    // packages/ sits two levels above oss/console/
    const packagesDir = path.resolve(__dirname, "../../packages")

    // Cache: package name → { dir, exportsMap }
    const pkgCache = new Map<string, { dir: string; exportsMap: Record<string, unknown> }>()

    function loadPkgExports(pkgName: string) {
        if (pkgCache.has(pkgName)) return pkgCache.get(pkgName)!

        const pkgDir = path.join(packagesDir, pkgName)
        const pkgJsonPath = path.join(pkgDir, "package.json")
        if (!fs.existsSync(pkgJsonPath)) return null

        const pkgJson = JSON.parse(fs.readFileSync(pkgJsonPath, "utf-8"))
        const result = {
            dir: pkgDir,
            exportsMap: (pkgJson.exports ?? {}) as Record<string, unknown>,
        }
        pkgCache.set(pkgName, result)
        return result
    }

    return {
        name: "enterprise-resolve",
        enforce: "pre",

        resolveId(source) {
            if (!source.startsWith(ENTERPRISE_PKG_PREFIX)) return

            // e.g. "@lunogram-enterprise/oapi-client/courier"
            //   → scope = "oapi-client", subpath = "/courier"
            // e.g. "@lunogram-enterprise/ai-builder"
            //   → scope = "ai-builder", subpath = ""
            const rest = source.slice(ENTERPRISE_PKG_PREFIX.length) // "oapi-client/courier"
            const slashIdx = rest.indexOf("/")
            const pkgName = slashIdx === -1 ? rest : rest.slice(0, slashIdx)
            const subpath = slashIdx === -1 ? "." : "./" + rest.slice(slashIdx + 1)

            const pkg = loadPkgExports(pkgName)
            if (!pkg) return // fall through to normal resolution

            const entry = pkg.exportsMap[subpath]
            if (!entry) return

            // exports entry is either a string or { types, default }
            const filePath =
                typeof entry === "string" ? entry : (entry as Record<string, string>).default
            if (!filePath) return

            return path.resolve(pkg.dir, filePath)
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
            // In enterprise builds, resolve @lunogram-enterprise/* imports
            // directly to source files in packages/. In OSS builds, stub
            // them out so Rollup doesn't choke on unresolvable imports.
            ...(isEnterprise ? [enterpriseResolvePlugin()] : [enterpriseStubPlugin()]),
            react(),
            tailwindcss(),
            faviconPlugin,
        ],
        server: {
            proxy: {
                // changeOrigin is deliberately off: the platform's CSRF guard
                // accepts a cookie-borne write whose Origin matches the request's
                // own Host, and rewriting Host to the proxy target breaks that
                // comparison for every write the console makes in development.
                "/api": {
                    target: env.VITE_PROXY_URL,
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
                "/preferences": {
                    target: env.VITE_PROXY_URL,
                    changeOrigin: true,
                },
                "/static": {
                    target: env.VITE_PROXY_URL,
                    changeOrigin: true,
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
            // Deduplicate React across the monorepo so that workspace
            // packages (e.g. block-editor) share the same React instance
            // as the host app. Without this, hooks like useRef crash with
            // "dispatcher is null" because two copies of React are loaded.
            dedupe: ["react", "react-dom"],
        },
    }
})
