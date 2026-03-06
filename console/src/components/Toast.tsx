import * as React from "react"
import type { Toast, ToastPosition, ToasterProps } from "react-hot-toast/headless"
import { resolveValue, useToaster } from "react-hot-toast/headless"
import clsx from "clsx"
import "./Toast.css"

// ToastIcon
const ToastIcon: React.FC<{ toast: Toast }> = ({ toast }) => {
    const { icon, type } = toast
    if (icon !== undefined) {
        if (typeof icon === "string") {
            return <div className="icon-wrapper">{icon}</div>
        } else {
            return icon
        }
    }

    if (type === "blank") {
        return null
    }

    return (
        <div className="indicator-wrapper">
            <div className="loader-icon" />
            {type !== "loading" && (
                <div className="status-wrapper">
                    {type === "error" ? (
                        <div className="error-icon" />
                    ) : (
                        <div className="checkmark-icon" />
                    )}
                </div>
            )}
        </div>
    )
}

// ToastBar
interface ToastBarProps {
    toast: Toast
    position?: ToastPosition
    style?: React.CSSProperties
}

const getAnimationStyle = (visible: boolean): React.CSSProperties => {
    return {
        animation: visible
            ? "toastBarEnter 0.35s cubic-bezier(.21,1.02,.73,1) forwards"
            : "toastBarExit 0.4s forwards cubic-bezier(.06,.71,.55,1)",
    }
}

function ToastBar({ toast, style }: ToastBarProps) {
    const animationStyle: React.CSSProperties = toast.height
        ? getAnimationStyle(toast.visible)
        : { opacity: 0 }

    const icon = <ToastIcon toast={toast} />
    const message = (
        <div {...toast.ariaProps} className="toast-message">
            {resolveValue(toast.message, toast)}
        </div>
    )

    return (
        <div
            className={clsx(toast.className, "ui-toast-bar")}
            style={{
                ...animationStyle,
                ...style,
                ...toast.style,
            }}
        >
            {icon}
            {message}
        </div>
    )
}

// ToastWrapper
interface ToastWrapperProps {
    id: string
    className?: string
    style?: React.CSSProperties
    onHeightUpdate: (id: string, height: number) => void
    children?: React.ReactNode
}

const ToastWrapper = ({ id, className, style, onHeightUpdate, children }: ToastWrapperProps) => {
    const ref = React.useCallback(
        (el: HTMLElement | null) => {
            if (el) {
                const updateHeight = () => {
                    const height = el.getBoundingClientRect().height
                    onHeightUpdate(id, height)
                }
                updateHeight()
                new MutationObserver(updateHeight).observe(el, {
                    subtree: true,
                    childList: true,
                    characterData: true,
                })
            }
        },
        [id, onHeightUpdate],
    )

    return (
        <div ref={ref} className={className} style={style}>
            {children}
        </div>
    )
}

// Position style helper
const getPositionStyle = (position: ToastPosition, offset: number): React.CSSProperties => {
    const top = position.includes("top")
    const verticalStyle: React.CSSProperties = top ? { top: 0 } : { bottom: 0 }
    const horizontalStyle: React.CSSProperties = position.includes("center")
        ? { justifyContent: "center" }
        : position.includes("right")
          ? { justifyContent: "flex-end" }
          : {}
    return {
        left: 0,
        right: 0,
        display: "flex",
        position: "absolute",
        transition: "all 230ms cubic-bezier(.21,1.02,.73,1)",
        transform: `translateY(${offset * (top ? 1 : -1)}px)`,
        ...verticalStyle,
        ...horizontalStyle,
    }
}

// Toaster
const DEFAULT_OFFSET = 16
export const Toaster: React.FC<ToasterProps> = ({
    reverseOrder,
    position = "top-center",
    toastOptions,
    gutter,
    children,
    containerStyle,
    containerClassName,
}) => {
    const { toasts, handlers } = useToaster(toastOptions)

    return (
        <div
            style={{
                position: "fixed",
                zIndex: 9999,
                top: DEFAULT_OFFSET,
                left: DEFAULT_OFFSET,
                right: DEFAULT_OFFSET,
                bottom: DEFAULT_OFFSET,
                pointerEvents: "none",
                ...containerStyle,
            }}
            className={containerClassName}
            onMouseEnter={handlers.startPause}
            onMouseLeave={handlers.endPause}
        >
            {toasts.map((t) => {
                const toastPosition = t.position ?? position
                const offset = handlers.calculateOffset(t, {
                    reverseOrder,
                    gutter,
                    defaultPosition: position,
                })
                const positionStyle = getPositionStyle(toastPosition, offset)

                return (
                    <ToastWrapper
                        id={t.id}
                        key={t.id}
                        onHeightUpdate={handlers.updateHeight}
                        className={t.visible ? "active" : ""}
                        style={positionStyle}
                    >
                        {t.type === "custom" ? (
                            resolveValue(t.message, t)
                        ) : children ? (
                            children(t)
                        ) : (
                            <ToastBar toast={t} position={toastPosition} />
                        )}
                    </ToastWrapper>
                )
            })}
        </div>
    )
}
