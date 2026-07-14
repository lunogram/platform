import type { Key, ReactNode } from "react"
import { useState } from "react"
import type { Modifier } from "react-popper"
import { usePopper } from "react-popper"

const modifiers: Array<Partial<Modifier<string, object>>> = [
    {
        name: "preventOverflow",
        enabled: true,
        options: {
            padding: 10,
        },
    },
    {
        name: "offset",
        options: {
            offset: [0, 4],
        },
    },
    {
        name: "sameWidth",
        enabled: true,
        phase: "beforeWrite",
        requires: ["computeStyles"],
        fn({ state }) {
            state.styles.popper.minWidth = `${state.rects.reference.width}px`
        },
        effect({ state }) {
            const reference = state.elements.reference as HTMLElement
            state.elements.popper.style.minWidth = `${reference.offsetWidth}px`
        },
    },
]

export function usePopperSelectDropdown() {
    const [referenceElement, setReferenceElement] = useState<HTMLElement | null>(null)
    const [popperElement, setPopperElement] = useState<HTMLElement | null>(null)
    const { styles, attributes } = usePopper(referenceElement, popperElement, {
        strategy: "fixed",
        placement: "bottom-start",
        modifiers,
    })

    return {
        setReferenceElement,
        setPopperElement,
        styles,
        attributes,
    }
}

export const defaultToValue = (option: unknown) => option

export const defaultGetValueKey = (option: unknown) =>
    (typeof option === "object" && option !== null
        ? ((option as Record<string, unknown>).id ?? (option as Record<string, unknown>).key)
        : option) as Key

export const defaultGetOptionDisplay = (option: unknown) =>
    (typeof option === "object" && option !== null
        ? ((option as Record<string, unknown>).label ?? (option as Record<string, unknown>).name)
        : option) as string

export const highlightSearch = (
    text: string,
    search: string,
    matchClassName = "match",
): ReactNode => {
    if (!text || !search) return text ?? ""

    const parts = text.split(search)
    if (parts.length === 1) return text

    return parts.flatMap((part, index) =>
        index < parts.length - 1
            ? [
                  part,
                  <strong key={`${matchClassName}-${index}`} className={matchClassName}>
                      {search}
                  </strong>,
              ]
            : [part],
    )
}

/**
 * Navigate from inside a Radix-based overlay (Sheet, Dialog, DropdownMenu).
 *
 * Radix sets `pointer-events: none` on `<body>` while an overlay is open.
 * When `navigate()` fires from inside the overlay, React Router unmounts the
 * component tree before Radix can run its cleanup, leaving the stale style
 * behind and making the entire page unclickable.
 *
 * Calling `setOpenMobile(false)` alone is not enough because the state update
 * is asynchronous — React Router's synchronous unmount wins the race.
 * We therefore manually strip the style before navigating.
 */
export function navigateFromOverlay(
    navigate: (to: string) => void,
    closeMobile: () => void,
    to: string,
) {
    closeMobile()
    document.body.style.removeProperty("pointer-events")
    navigate(to)
}
