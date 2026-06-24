import { useState } from "preact/hooks";
import type { PreviewMode } from "./main";

interface Props {
  mode: PreviewMode;
  actionType: string;
  config: Record<string, any>;
  payload: Record<string, any>;
  functionId: string;
  input: Record<string, any>;
}

const METHOD_COLORS: Record<string, string> = {
  GET: "bg-green text-crust",
  POST: "bg-blue text-crust",
  PUT: "bg-peach text-crust",
  PATCH: "bg-mauve text-crust",
  DELETE: "bg-red text-crust",
};

function buildCurl(
  method: string,
  url: string,
  headers: Record<string, string>,
  body: unknown,
): string {
  const parts = [];

  if (method !== "GET") {
    parts.push(`-X ${method}`);
  }

  parts.push(url ? `'${url}'` : "'<endpoint>'");

  for (const [key, value] of Object.entries(headers)) {
    parts.push(`-H '${key}: ${value}'`);
  }

  if (body && method !== "GET") {
    const bodyStr = typeof body === "string" ? body : JSON.stringify(body);
    parts.push(`-d '${bodyStr}'`);
  }

  return "curl " + parts.join(" \\\n  ");
}

/**
 * Resolve the webhook-specific fields from the function's input object
 * (function-call mode).
 */
function resolveFields(props: Props) {
  const src = props.input ?? {};
  return {
    method: ((src.method as string) ?? "GET").toUpperCase(),
    url: (src.endpoint ?? src.url ?? "") as string,
    headers: (src.headers as Record<string, string>) ?? {},
    body: src.body,
  };
}

const CAPABILITIES = [
  { label: "HTTP Methods", detail: "GET, POST, PUT, PATCH, DELETE" },
  { label: "Custom Headers", detail: "Add authentication and custom headers" },
  { label: "Request Body", detail: "Send JSON or raw payloads" },
  { label: "Dynamic Input", detail: "Map journey data into each request" },
];

function ActionConfigPreview() {
  return (
    <div class="flex flex-col gap-0.5">
      <div class="bg-mantle px-3.5 py-4 rounded-t-lg">
        <div class="text-sm font-semibold text-text mb-2">Webhook Action</div>
        <p class="text-[13px] text-subtext0 leading-relaxed m-0">
          Make HTTP requests to any external API or service. Configure the
          base URL and default headers here, then define per-step request
          details inside your journeys.
        </p>
      </div>
      <div class="bg-mantle px-3.5 py-3">
        <div class="flex justify-between items-center mb-1.5">
          <div class="text-[10px] uppercase tracking-wider text-overlay0 font-semibold">
            Capabilities
          </div>
        </div>
        <ul class="list-none m-0 p-0 flex flex-col gap-2">
          {CAPABILITIES.map((cap) => (
            <li key={cap.label} class="flex items-baseline gap-2.5 text-xs">
              <span class="text-text font-semibold whitespace-nowrap">
                {cap.label}
              </span>
              <span class="text-overlay0">{cap.detail}</span>
            </li>
          ))}
        </ul>
      </div>
      <div class="bg-mantle px-3.5 py-3 rounded-b-lg">
        <div class="flex justify-between items-center mb-1.5">
          <div class="text-[10px] uppercase tracking-wider text-overlay0 font-semibold">
            Example
          </div>
        </div>
        <pre class="block bg-crust rounded px-3 py-2.5 text-xs overflow-x-auto overflow-y-auto max-h-[300px] whitespace-pre-wrap break-all text-text m-0">
          {`curl -X POST 'https://api.example.com/endpoint' \\
  -H 'Content-Type: application/json' \\
  -d '{"key": "value"}'`}
        </pre>
      </div>
    </div>
  );
}

function MethodBadge({
  method,
  hasMethod,
}: {
  method: string;
  hasMethod: boolean;
}) {
  if (!hasMethod) {
    return (
      <span class="inline-block px-2 py-0.5 rounded text-xs font-bold text-overlay1 bg-surface0 mr-2.5 align-middle">
        {method}
      </span>
    );
  }
  return (
    <span
      class={`inline-block px-2 py-0.5 rounded text-xs font-bold mr-2.5 align-middle ${METHOD_COLORS[method] ?? ""}`}
    >
      {method}
    </span>
  );
}

export function WebhookPreview(props: Props) {
  if (props.mode === "action-config") {
    return <ActionConfigPreview />;
  }

  const [copied, setCopied] = useState(false);
  const { method, url, headers, body } = resolveFields(props);

  const hasUrl = url !== "";
  const hasMethod = !!(props.input?.method);

  return (
    <div class="flex flex-col gap-0.5">
      {/* Method + URL */}
      <div class="bg-mantle px-3.5 py-3 rounded-t-lg">
        <div>
          <MethodBadge method={method} hasMethod={hasMethod} />
          {hasUrl ? (
            <span class="text-teal break-all align-middle text-[13px]">
              {url}
            </span>
          ) : (
            <span class="text-surface1 italic text-xs">
              https://api.example.com/endpoint
            </span>
          )}
        </div>
      </div>

      {/* curl snippet */}
      <div class="bg-mantle px-3.5 py-3 rounded-b-lg">
        <div class="flex justify-between items-center mb-1.5">
          <div class="text-[10px] uppercase tracking-wider text-overlay0 font-semibold">
            curl
          </div>
          <button
            class={`bg-surface0 border-none rounded text-xs px-2 py-0.5 cursor-pointer font-[inherit] leading-snug ${
              copied ? "text-green" : "text-text"
            }`}
            onClick={() => {
              const curl = buildCurl(method, url, headers, body);
              window.parent.postMessage(
                { type: "copy-to-clipboard", text: curl },
                "*",
              );
              setCopied(true);
              setTimeout(() => setCopied(false), 2000);
            }}
          >
            {copied ? "Copied!" : "Copy"}
          </button>
        </div>
        <pre class="block bg-crust rounded px-3 py-2.5 text-xs overflow-x-auto overflow-y-auto max-h-[300px] whitespace-pre-wrap break-all text-text m-0">
          {buildCurl(method, url, headers, body)}
        </pre>
      </div>
    </div>
  );
}
