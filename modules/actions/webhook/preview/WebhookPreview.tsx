import type { JSX } from 'preact'

interface Props {
  actionType: string
  config: Record<string, any>
  payload: Record<string, any>
  variables: Record<string, string>
}

const METHOD_COLORS: Record<string, string> = {
  GET: '#a6e3a1',
  POST: '#89b4fa',
  PUT: '#fab387',
  PATCH: '#cba6f7',
  DELETE: '#f38ba8',
}

const styles = {
  container: {
    display: 'flex',
    flexDirection: 'column',
    gap: '2px',
  } as JSX.CSSProperties,
  section: {
    background: '#181825',
    padding: '12px 14px',
  } as JSX.CSSProperties,
  sectionFirst: {
    borderTopLeftRadius: '8px',
    borderTopRightRadius: '8px',
  } as JSX.CSSProperties,
  sectionLast: {
    borderBottomLeftRadius: '8px',
    borderBottomRightRadius: '8px',
  } as JSX.CSSProperties,
  label: {
    fontSize: '10px',
    textTransform: 'uppercase',
    letterSpacing: '0.08em',
    color: '#6c7086',
    marginBottom: '6px',
    fontWeight: 600,
  } as JSX.CSSProperties,
  badge: (color: string) =>
    ({
      display: 'inline-block',
      padding: '2px 8px',
      borderRadius: '4px',
      fontSize: '12px',
      fontWeight: 700,
      color: '#1e1e2e',
      backgroundColor: color,
      marginRight: '10px',
      verticalAlign: 'middle',
    }) as JSX.CSSProperties,
  badgeEmpty: {
    display: 'inline-block',
    padding: '2px 8px',
    borderRadius: '4px',
    fontSize: '12px',
    fontWeight: 700,
    color: '#585b70',
    backgroundColor: '#313244',
    marginRight: '10px',
    verticalAlign: 'middle',
  } as JSX.CSSProperties,
  url: {
    color: '#89dceb',
    wordBreak: 'break-all',
    verticalAlign: 'middle',
    fontSize: '13px',
  } as JSX.CSSProperties,
  table: {
    width: '100%',
    borderCollapse: 'collapse',
    fontSize: '12px',
  } as JSX.CSSProperties,
  th: {
    textAlign: 'left',
    padding: '4px 8px',
    borderBottom: '1px solid #313244',
    color: '#6c7086',
    fontWeight: 600,
    fontSize: '11px',
  } as JSX.CSSProperties,
  td: {
    padding: '4px 8px',
    borderBottom: '1px solid #313244',
    color: '#cdd6f4',
  } as JSX.CSSProperties,
  code: {
    display: 'block',
    background: '#11111b',
    borderRadius: '4px',
    padding: '10px 12px',
    fontSize: '12px',
    overflowX: 'auto',
    whiteSpace: 'pre-wrap',
    wordBreak: 'break-all',
    color: '#cdd6f4',
    margin: 0,
  } as JSX.CSSProperties,
  liquid: {
    color: '#94e2d5',
    fontWeight: 600,
  } as JSX.CSSProperties,
  placeholder: {
    color: '#45475a',
    fontStyle: 'italic',
    fontSize: '12px',
  } as JSX.CSSProperties,
  emptyTable: {
    color: '#45475a',
    fontStyle: 'italic',
    fontSize: '12px',
    padding: '6px 8px',
  } as JSX.CSSProperties,
} as const

function highlightLiquid(text: string): (string | JSX.Element)[] {
  const parts: (string | JSX.Element)[] = []
  const regex = /(\{\{.*?\}\})/g
  let last = 0
  let match: RegExpExecArray | null

  while ((match = regex.exec(text)) !== null) {
    if (match.index > last) {
      parts.push(text.slice(last, match.index))
    }
    parts.push(
      <span style={styles.liquid}>{match[1]}</span>
    )
    last = regex.lastIndex
  }
  if (last < text.length) {
    parts.push(text.slice(last))
  }
  return parts
}

function formatJson(value: unknown): string {
  if (value === null || value === undefined) return ''
  if (typeof value === 'string') {
    try {
      return JSON.stringify(JSON.parse(value), null, 2)
    } catch {
      return value
    }
  }
  return JSON.stringify(value, null, 2)
}

function buildCurl(config: Record<string, any>): string {
  const method = (config.method ?? 'GET').toUpperCase()
  const url = config.url ?? config.endpoint ?? ''
  const headers: Record<string, string> = config.headers ?? {}
  const body = config.body

  const parts = ['curl']

  if (method !== 'GET') {
    parts.push(`-X ${method}`)
  }

  parts.push(url ? `'${url}'` : "'<endpoint>'")

  for (const [key, value] of Object.entries(headers)) {
    parts.push(`-H '${key}: ${value}'`)
  }

  if (body && method !== 'GET') {
    const bodyStr = typeof body === 'string' ? body : JSON.stringify(body)
    parts.push(`-d '${bodyStr}'`)
  }

  return parts.join(' \\\n  ')
}

export function WebhookPreview({ config, variables }: Props) {
  const method = ((config.method as string) ?? 'GET').toUpperCase()
  const url = (config.url ?? config.endpoint ?? '') as string
  const headers: Record<string, string> = (config.headers as Record<string, string>) ?? {}
  const body = config.body
  const headerEntries = Object.entries(headers)
  const variableEntries = Object.entries(variables ?? {})

  const hasUrl = url !== ''
  const hasHeaders = headerEntries.length > 0
  const hasBody = body !== null && body !== undefined && body !== ''
  const hasVariables = variableEntries.length > 0

  const methodColor = METHOD_COLORS[method] ?? '#cdd6f4'
  const hasMethod = config.method !== undefined && config.method !== ''

  return (
    <div style={styles.container}>
      {/* Method + URL */}
      <div style={{ ...styles.section, ...styles.sectionFirst }}>
        <div>
          {hasMethod ? (
            <span style={styles.badge(methodColor)}>{method}</span>
          ) : (
            <span style={styles.badgeEmpty}>GET</span>
          )}
          {hasUrl ? (
            <span style={styles.url}>{highlightLiquid(url)}</span>
          ) : (
            <span style={styles.placeholder}>https://api.example.com/endpoint</span>
          )}
        </div>
      </div>

      {/* Headers */}
      <div style={styles.section}>
        <div style={styles.label}>Headers</div>
        {hasHeaders ? (
          <table style={styles.table}>
            <thead>
              <tr>
                <th style={styles.th}>Name</th>
                <th style={styles.th}>Value</th>
              </tr>
            </thead>
            <tbody>
              {headerEntries.map(([key, value]) => (
                <tr key={key}>
                  <td style={styles.td}>{key}</td>
                  <td style={styles.td}>{highlightLiquid(String(value))}</td>
                </tr>
              ))}
            </tbody>
          </table>
        ) : (
          <div style={styles.emptyTable}>No headers configured</div>
        )}
      </div>

      {/* Body */}
      <div style={styles.section}>
        <div style={styles.label}>Body</div>
        {hasBody ? (
          <pre style={styles.code}>{highlightLiquid(formatJson(body))}</pre>
        ) : (
          <div style={styles.emptyTable}>No request body</div>
        )}
      </div>

      {/* Variables */}
      <div style={styles.section}>
        <div style={styles.label}>Variables</div>
        {hasVariables ? (
          <table style={styles.table}>
            <thead>
              <tr>
                <th style={styles.th}>Name</th>
                <th style={styles.th}>Value</th>
              </tr>
            </thead>
            <tbody>
              {variableEntries.map(([name, expr]) => (
                <tr key={name}>
                  <td style={styles.td}>{name}</td>
                  <td style={styles.td}>{highlightLiquid(String(expr))}</td>
                </tr>
              ))}
            </tbody>
          </table>
        ) : (
          <div style={styles.emptyTable}>No variables defined</div>
        )}
      </div>

      {/* curl snippet — always visible */}
      <div style={{ ...styles.section, ...styles.sectionLast }}>
        <div style={styles.label}>curl</div>
        <pre style={styles.code}>{highlightLiquid(buildCurl(config))}</pre>
      </div>
    </div>
  )
}
