import { render } from 'preact'
import { useState, useEffect, useRef, useCallback } from 'preact/hooks'
import { WebhookPreview } from './WebhookPreview'

interface PreviewState {
  actionType: string
  config: Record<string, any>
  payload: Record<string, any>
  variables: Record<string, string>
}

/** Post the current height to the parent frame */
function postHeight() {
  // Use scrollHeight on the document to capture the full content height
  // including margins that collapse outside the root div
  const height = Math.max(
    document.documentElement.scrollHeight,
    document.body.scrollHeight,
  )
  window.parent.postMessage({ type: 'resize', height }, '*')
}

function App() {
  const [state, setState] = useState<PreviewState>({
    actionType: '',
    config: {},
    payload: {},
    variables: {},
  })
  const rootRef = useRef<HTMLDivElement>(null)
  const rafRef = useRef<number>(0)

  // Debounced height post — coalesce rapid ResizeObserver callbacks
  // into a single rAF
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
          actionType: data.actionType ?? '',
          config: data.config ?? {},
          payload: data.payload ?? {},
          variables: data.variables ?? {},
        })
      }
    }
    window.addEventListener('message', handler)
    // Signal to the parent that the listener is ready
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

  // Post height after every render (state change triggers re-render,
  // which may change content before ResizeObserver fires)
  useEffect(() => {
    // Use rAF so layout has completed before measuring
    requestAnimationFrame(postHeight)
  })

  // Post initial height once the page has loaded
  useEffect(() => {
    postHeight()
  }, [])

  return (
    <div ref={rootRef}>
      <WebhookPreview
        actionType={state.actionType}
        config={state.config}
        payload={state.payload}
        variables={state.variables}
      />
    </div>
  )
}

render(<App />, document.getElementById('root')!)
