import { useCallback, useEffect, useMemo, useRef, useState } from "react"

export function useResolver<T>(resolver: () => Promise<T>) {
    const [value, setValue] = useState<null | T>(null)
    // loading tracks whether a request is in flight. It starts true so the first
    // render shows a loading state, and — crucially — flips to false once the
    // request settles even when it resolves empty or rejects, so callers never
    // get stuck on a skeleton when there is no data or the request fails.
    const [loading, setLoading] = useState(true)
    // error is the rejection of the latest request, cleared once one succeeds.
    const [error, setError] = useState<unknown>(undefined)
    // Only the latest call may commit: a slow response for old params (or an
    // old resolver) must not overwrite a newer one.
    const latestCall = useRef(0)
    const reload = useCallback(async () => {
        const call = ++latestCall.current
        setLoading(true)
        try {
            const next = await resolver()
            if (call !== latestCall.current) return
            setValue(next)
            setError(undefined)
        } catch (err) {
            if (call !== latestCall.current) return
            console.error(err)
            setError(err)
        } finally {
            if (call === latestCall.current) setLoading(false)
        }
    }, [resolver])
    useEffect(() => {
        reload().catch((err) => console.error(err))
    }, [reload])
    useEffect(
        () => () => {
            // Nothing may commit after unmount.
            latestCall.current++
        },
        [],
    )
    // error is last so existing callers' destructuring keeps working.
    return useMemo(
        () => [value, setValue, reload, loading, error] as const,
        [value, reload, loading, error],
    )
}

export function useDebounceControl<T>(value: T, onChange: (value: T) => void, ms = 400) {
    const changeRef = useRef(onChange)
    changeRef.current = onChange
    const valueRef = useRef(value)
    valueRef.current = value
    const timeoutId = useRef<ReturnType<typeof setTimeout>>()
    const synced = useRef(true)
    const [temp, setTemp] = useState<T>(value)
    useEffect(() => {
        clearTimeout(timeoutId.current)
        if (valueRef.current !== temp) {
            timeoutId.current = setTimeout(() => {
                changeRef.current(temp)
                synced.current = false
            }, ms)
        }
    }, [temp, ms])
    useEffect(() => {
        if (!synced.current) {
            setTemp(value)
            synced.current = true
        }
    }, [value])
    return [temp, setTemp] as const
}
