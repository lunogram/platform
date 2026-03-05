import type { JSX } from "preact";
import { useState } from "preact/hooks";
import "./WebhookPreview.css";

interface Props {
  actionType: string;
  config: Record<string, any>;
  payload: Record<string, any>;
  variables: Record<string, string>;
}

const METHOD_CLASS: Record<string, string> = {
  GET: "badge--get",
  POST: "badge--post",
  PUT: "badge--put",
  PATCH: "badge--patch",
  DELETE: "badge--delete",
};

function highlightLiquid(text: string): (string | JSX.Element)[] {
  const parts: (string | JSX.Element)[] = [];
  const regex = /(\{\{.*?\}\})/g;
  let last = 0;
  let match: RegExpExecArray | null;

  while ((match = regex.exec(text)) !== null) {
    if (match.index > last) {
      parts.push(text.slice(last, match.index));
    }
    parts.push(<span class="liquid">{match[1]}</span>);
    last = regex.lastIndex;
  }
  if (last < text.length) {
    parts.push(text.slice(last));
  }
  return parts;
}

function formatJson(value: unknown): string {
  if (value === null || value === undefined) return "";
  if (typeof value === "string") {
    try {
      return JSON.stringify(JSON.parse(value), null, 2);
    } catch {
      return value;
    }
  }
  return JSON.stringify(value, null, 2);
}

function buildCurl(config: Record<string, any>): string {
  const method = (config.method ?? "GET").toUpperCase();
  const url = config.url ?? config.endpoint ?? "";
  const headers: Record<string, string> = config.headers ?? {};
  const body = config.body;

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

function KeyValueTable({
  entries,
  emptyMessage,
}: {
  entries: [string, string][];
  emptyMessage: string;
}) {
  if (entries.length === 0) {
    return <div class="empty-table">{emptyMessage}</div>;
  }

  return (
    <table class="table">
      <thead>
        <tr>
          <th class="th">Name</th>
          <th class="th">Value</th>
        </tr>
      </thead>
      <tbody>
        {entries.map(([key, value]) => (
          <tr key={key}>
            <td class="td">{key}</td>
            <td class="td">{highlightLiquid(String(value))}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

export function WebhookPreview({ config, variables }: Props) {
  const [copied, setCopied] = useState(false);

  const method = ((config.method as string) ?? "GET").toUpperCase();
  const url = (config.url ?? config.endpoint ?? "") as string;
  const headers: Record<string, string> =
    (config.headers as Record<string, string>) ?? {};
  const body = config.body;
  const headerEntries = Object.entries(headers);
  const variableEntries = Object.entries(variables ?? {});

  const hasUrl = url !== "";
  const hasBody = body !== null && body !== undefined && body !== "";
  const hasMethod = config.method !== undefined && config.method !== "";

  const badgeClass = hasMethod
    ? `badge ${METHOD_CLASS[method] ?? ""}`
    : "badge--empty";

  return (
    <div class="container">
      {/* Method + URL */}
      <div class="section">
        <div>
          <span class={badgeClass}>{method}</span>
          {hasUrl ? (
            <span class="url">{highlightLiquid(url)}</span>
          ) : (
            <span class="placeholder">https://api.example.com/endpoint</span>
          )}
        </div>
      </div>

      {/* Headers */}
      <div class="section">
        <div class="label">Headers</div>
        <KeyValueTable
          entries={headerEntries as [string, string][]}
          emptyMessage="No headers configured"
        />
      </div>

      {/* Body */}
      <div class="section">
        <div class="label">Body</div>
        {hasBody ? (
          <pre class="code">{highlightLiquid(formatJson(body))}</pre>
        ) : (
          <div class="empty-table">No request body</div>
        )}
      </div>

      {/* Variables */}
      <div class="section">
        <div class="label">Variables</div>
        <KeyValueTable
          entries={variableEntries as [string, string][]}
          emptyMessage="No variables defined"
        />
      </div>

      {/* curl snippet — always visible */}
      <div class="section">
        <div class="section-header">
          <div class="section-header__label">curl</div>
          <button
            class={`copy-button${copied ? " copy-button--copied" : ""}`}
            onClick={() => {
              const curl = buildCurl(config);
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
        <pre class="code">{highlightLiquid(buildCurl(config))}</pre>
      </div>
    </div>
  );
}
