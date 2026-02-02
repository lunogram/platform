import * as React from 'react'
import type { NavLinkProps as BaseNavLinkProps } from 'react-router';
import { NavLink as BaseNavLink } from 'react-router'

export type NavLinkProps = BaseNavLinkProps & { icon?: React.ReactNode }

const NavLink = React.forwardRef(
    function NavLink({ icon, ...props }: NavLinkProps, ref: React.Ref<HTMLAnchorElement> | undefined) {
        return (
            <BaseNavLink
                ref={ref}
                {...props}
                className={({ isActive }: { isActive: boolean }) =>
                    [
                        props.className,
                        isActive ? 'selected' : null,
                    ]
                        .filter(Boolean)
                        .join(' ')
                }
            >
                <>
                    {icon && (<div className="nav-icon">{icon}</div>)}
                    {props.children}
                </>
            </BaseNavLink>
        )
    },
)

export default NavLink
