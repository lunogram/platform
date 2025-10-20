import type { PropsWithChildren } from 'react'
import clsx from 'clsx'

import './Tag.css'

export type TagVariant = 'info' | 'plain' | 'success' | 'error' | 'warn'
export type TagSize = 'tiny' | 'regular' | 'large'

export type TagProps = PropsWithChildren<{
    onClick?: () => void
    children?: React.ReactNode
    variant?: TagVariant
    size?: TagSize
}>

export default function Tag({ variant = 'info', size = 'regular', children, onClick }: TagProps) {
    return (
        <div className={clsx('ui-tag', variant, size)}>
            {children}
            {onClick && <div className="tag-close bi-x" onClick={onClick} />}
        </div>
    )
}

export function TagGroup({ children }: PropsWithChildren) {
    return <div className="ui-tag-group">{children}</div>
}
