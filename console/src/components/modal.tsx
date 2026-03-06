import {
    Dialog,
    DialogPanel,
    DialogTitle,
    DialogBackdrop,
    Transition,
    TransitionChild,
} from "@headlessui/react"
import type { PropsWithChildren, ReactNode } from "react"
import { Fragment } from "react"
import { Button } from "@/components/ui/button"
import { CloseIcon } from "@/components/icons"
import { useTranslation } from "react-i18next"
import { cn } from "@/utils"

export interface ModalStateProps {
    open: boolean
    onClose: (open: boolean) => void
}

export interface ModalProps extends ModalStateProps {
    title: ReactNode
    description?: ReactNode
    actions?: ReactNode
    size?: "small" | "regular" | "large" | "fullscreen"
    zIndex?: number
}

const sizeStyles = {
    small: "max-w-[440px]",
    regular: "max-w-[600px]",
    large: "max-w-[960px]",
    fullscreen: "max-w-full max-h-screen h-screen w-screen rounded-none p-0 flex flex-col",
}

export default function Modal({
    children,
    description,
    open,
    onClose,
    title,
    actions,
    size = "small",
    zIndex = 999,
}: PropsWithChildren<ModalProps>) {
    const { t } = useTranslation()
    return (
        <Transition show={open} as={Fragment}>
            <Dialog as="div" className="fixed inset-0" onClose={onClose} style={{ zIndex }}>
                <TransitionChild
                    as={Fragment}
                    enter="transition-opacity duration-100 ease-out"
                    enterFrom="opacity-0"
                    enterTo="opacity-100"
                    leave="transition-opacity duration-100 ease-in"
                    leaveFrom="opacity-100"
                    leaveTo="opacity-0"
                >
                    <DialogBackdrop className="fixed inset-0 bg-black/30" style={{ zIndex }} />
                </TransitionChild>
                <div
                    className="absolute inset-0 flex items-center justify-center"
                    style={{ zIndex: zIndex + 1 }}
                >
                    <TransitionChild
                        as={Fragment}
                        enter="transition-all duration-100 ease-out"
                        enterFrom="opacity-0 scale-105"
                        enterTo="opacity-100 scale-100"
                        leave="transition-all duration-100 ease-in"
                        leaveFrom="opacity-100 scale-100"
                        leaveTo="opacity-0"
                    >
                        <DialogPanel
                            className={cn(
                                "w-full rounded-xl bg-background p-8 shadow-xl max-h-[90vh] overflow-y-auto relative",
                                sizeStyles[size],
                            )}
                        >
                            <div
                                className={cn(
                                    "mb-5 flex flex-row items-center",
                                    size === "fullscreen" &&
                                        "mb-0 px-5 py-2.5 border-b border-border [&>*+*]:ml-4",
                                )}
                            >
                                {size === "fullscreen" && (
                                    <Button variant="secondary" onClick={() => onClose(false)}>
                                        <CloseIcon />
                                        {t("exit")}
                                    </Button>
                                )}
                                <DialogTitle
                                    as="h3"
                                    className="m-0 flex-grow font-semibold tracking-tight"
                                >
                                    {title}
                                </DialogTitle>
                                {size === "fullscreen" && actions && (
                                    <div className="flex gap-2.5 items-center">{actions}</div>
                                )}
                            </div>
                            {description && (
                                <p className="mb-4 text-muted-foreground">{description}</p>
                            )}
                            <div
                                className={cn(
                                    size === "fullscreen" && "flex-grow relative overflow-auto",
                                )}
                            >
                                {children}
                            </div>
                            {!!(actions && size !== "fullscreen") && (
                                <div className="mt-5">{actions}</div>
                            )}
                            {size !== "fullscreen" && (
                                <Button
                                    className="absolute top-8 right-6"
                                    size="sm"
                                    variant="ghost"
                                    onClick={() => onClose(false)}
                                >
                                    <CloseIcon />
                                </Button>
                            )}
                        </DialogPanel>
                    </TransitionChild>
                </div>
            </Dialog>
        </Transition>
    )
}
