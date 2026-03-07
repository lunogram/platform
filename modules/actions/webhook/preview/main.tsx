import { render } from 'preact'
import { useState, useEffect, useRef, useCallback } from 'preact/hooks'
import { WebhookPreview } from './WebhookPreview'

export type PreviewMode = 'action-config' | 'function-call'

interface PreviewState {
  mode: PreviewMode
  actionType: string
  config: Record<string, any>
  payload: Record<string, any>
  functionId: string
  input: Record<string, any>
}

/** Post the current height to the parent frame */
function postHeight() {
  const height = Math.max(
    document.documentElement.scrollHeight,
    document.body.scrollHeight,
  )
  window.parent.postMessage({ type: 'resize', height }, '*')
}

function App() {
  const [state, setState] = useState<PreviewState>({
    mode: 'action-config',
    actionType: '',
    config: {},
    payload: {},
    functionId: '',
    input: {},
  })
  const rootRef = useRef<HTMLDivElement>(null)
  const rafRef = useRef<number>(0)

  const debouncedPostHeight = useCallback(() => {
    if (rafRef.current) cancelAnimationFrame(rafRef.current)
    rafRef.current = requestAnimationFrame(() => {
      postHeight()
    })
  }, [])

  useEffect(() => {
    const handler = (event: MessageEvent) => {
      const data = event.data
      if (data && data.type === 'preview-update') {
        setState({
          mode: data.mode ?? 'action-config',
          actionType: data.actionType ?? '',
          config: data.config ?? {},
          payload: data.payload ?? {},
          functionId: data.functionId ?? '',
          input: data.input ?? {},
        })
      }
    }
    window.addEventListener('message', handler)
    window.parent.postMessage({ type: 'preview-ready' }, '*')
    return () => window.removeEventListener('message', handler)
  }, [])

  // Observe the root div for size changes
  useEffect(() => {
    const el = rootRef.current
    if (!el) return

    const observer = new ResizeObserver(() => {
      debouncedPostHeight()
    })
    observer.observe(el)
    return () => observer.disconnect()
  }, [debouncedPostHeight])

  // Post height after every render
  useEffect(() => {
    requestAnimationFrame(postHeight)
  })

  // Post initial height once the page has loaded
  useEffect(() => {
    postHeight()
  }, [])

  return (
    <div ref={rootRef}>
      <WebhookPreview
        mode={state.mode}
        actionType={state.actionType}
        config={state.config}
        payload={state.payload}
        functionId={state.functionId}
        input={state.input}
      />
    </div>
  )
}

render(<App />, document.getElementById('root')!)
