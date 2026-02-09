import { Dialog, DialogPanel, DialogTitle, DialogBackdrop, Transition, TransitionChild } from '@headlessui/react'
import type { PropsWithChildren, ReactNode } from 'react';
import { Fragment } from 'react'
import { Button } from '@/components/ui/button'
import { CloseIcon } from '../components/icons'
import { useTranslation } from 'react-i18next'
import './Modal.css'

export interface ModalStateProps {
    open: boolean
    onClose: (open: boolean) => void
}

export interface ModalProps extends ModalStateProps {
    title: ReactNode
    description?: ReactNode
    actions?: ReactNode
    size?: 'small' | 'regular' | 'large' | 'fullscreen'
    zIndex?: number
}

export default function Modal({
    children,
    description,
    open,
    onClose,
    title,
    actions,
    size,
    zIndex = 999,
}: PropsWithChildren<ModalProps>) {
    const { t } = useTranslation()
    return (
        <Transition show={open} as={Fragment}>
            <Dialog
                as="div"
                className={`modal ${size ?? 'small'}`}
                onClose={onClose}
                style={{ zIndex }}
            >
                <TransitionChild
                    as={Fragment}
                    enter="transition-enter"
                    enterFrom="transition-enter-from"
                    enterTo="transition-enter-to"
                    leave="transition-leave"
                    leaveFrom="transition-leave-from"
                    leaveTo="transition-leave-to"
                >
                    <DialogBackdrop className="modal-overlay" style={{ zIndex }} />
                </TransitionChild>
                <div className="modal-wrapper" style={{ zIndex: zIndex + 1 }}>
                    <TransitionChild
                        as={Fragment}
                        enter="transition-enter"
                        enterFrom="transition-enter-from transition-enter-from-scale"
                        enterTo="transition-enter-to"
                        leave="transition-leave"
                        leaveFrom="transition-leave-from"
                        leaveTo="transition-leave-to"
                    >
                        <DialogPanel className="modal-inner">
                            <div className="modal-header">
                                {
                                    size === 'fullscreen' && (
                                        <Button
                                            variant="secondary"
                                            onClick={() => onClose(false)}
                                        >
                                            <CloseIcon />
                                            {t('exit')}
                                        </Button>
                                    )
                                }
                                <DialogTitle as="h3">{title}</DialogTitle>
                                {
                                    size === 'fullscreen' && actions && (
                                        <div className="modal-fullscreen-actions">
                                            {actions}
                                        </div>
                                    )
                                }
                            </div>
                            {
                                description && (
                                    <p className="modal-description">
                                        {description}
                                    </p>
                                )
                            }
                            <div className="modal-content">
                                {children}
                            </div>
                            {
                                !!(actions && size !== 'fullscreen') && (
                                    <div className="modal-footer">
                                        {actions}
                                    </div>
                                )
                            }
                            {
                                size !== 'fullscreen' && (
                                    <Button
                                        className="modal-close"
                                        size="sm"
                                        variant="ghost"
                                        onClick={() => onClose(false)}
                                    >
                                        <CloseIcon />
                                    </Button>
                                )
                            }
                        </DialogPanel>
                    </TransitionChild>
                </div>
            </Dialog>
        </Transition>
    )
}
