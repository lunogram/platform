import { useCallback, useEffect, useMemo, useRef, useState } from "react"

export function useResolver<T>(resolver: () => Promise<T>) {
    const [value, setValue] = useState<null | T>(null)
    // loading tracks whether a request is in flight. It starts true so the first
    // render shows a loading state, and — crucially — flips to false once the
    // request settles even when it resolves empty or rejects, so callers never
    // get stuck on a skeleton when there is no data or the request fails.
    const [loading, setLoading] = useState(true)
    const reload = useCallback(async () => {
        setLoading(true)
        try {
            setValue(await resolver())
        } catch (err) {
            console.error(err)
        } finally {
            setLoading(false)
        }
    }, [resolver])
    useEffect(() => {
        reload().catch((err) => console.error(err))
    }, [reload])
    return useMemo(() => [value, setValue, reload, loading] as const, [value, reload, loading])
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
