import { useContext, useState } from 'react'
import { Outlet, useNavigate, NavLink, useLocation, Link } from 'react-router'
import { useTranslation } from 'react-i18next'
import {
    UserCircle2,
    Trash2,
    FileText,
    Activity,
    Bell,
    Route,
    Building2,
    ChevronRight,
    MoreHorizontal,
} from 'lucide-react'
import { ProjectContext, UserContext } from '../../contexts'
import { PreferencesContext } from '../../ui/PreferencesContext'
import { getRandomColor } from '@/lib/colors'
import { formatDate, cn } from '../../utils'
import api from '../../api'

import { Button } from '@/components/ui/button'
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from '@/components/ui/dialog'
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'

export default function UserDetail() {
    const { t } = useTranslation()
    const navigate = useNavigate()
    const location = useLocation()
    const [preferences] = useContext(PreferencesContext)
    const [project] = useContext(ProjectContext)
    const [user] = useContext(UserContext)
    const [isDeleteOpen, setIsDeleteOpen] = useState(false)
    const [isDeleting, setIsDeleting] = useState(false)

    const userColor = getRandomColor(user.email ?? user.external_id ?? user.id)

    const displayName = user.full_name
        ?? (user.data as Record<string, unknown>)?.full_name as string
        ?? user.email
        ?? 'No name'

    const initials = (() => {
        const parts = displayName.split(/[\s@.]+/)
        if (parts.length >= 2) {
            return (parts[0][0] + parts[1][0]).toUpperCase()
        }
        return displayName.substring(0, 2).toUpperCase()
    })()

    // Determine active tab
    const basePath = `/projects/${project.id}/users/${user.id}`
    const currentPath = location.pathname
    const activeTab = currentPath === basePath ? 'details' : currentPath.split('/').pop()

    const deleteUser = async () => {
        setIsDeleting(true)
        try {
            await api.users.delete(project.id, user.id)
            await navigate(`/projects/${project.id}/users`)
        } finally {
            setIsDeleting(false)
        }
    }

    const tabs = [
        { key: 'details', to: '', label: t('details'), icon: FileText },
        { key: 'events', to: 'events', label: t('events'), icon: Activity },
        { key: 'subscriptions', to: 'subscriptions', label: t('subscriptions'), icon: Bell },
        { key: 'journeys', to: 'journeys', label: t('journeys'), icon: Route },
        { key: 'organizations', to: 'organizations', label: t('organizations'), icon: Building2 },
    ]

    return (
        <div className="flex flex-col min-h-full">
            {/* Header Section */}
            <div className="border-b bg-card/50">
                <div className="p-6 pb-0">
                    {/* Breadcrumb */}
                    <nav className="flex items-center gap-1.5 text-sm text-muted-foreground mb-4">
                        <Link
                            to={`/projects/${project.id}/users`}
                            className="hover:text-foreground transition-colors"
                        >
                            {t('users')}
                        </Link>
                        <ChevronRight className="h-3.5 w-3.5" />
                        <span className="text-foreground font-medium">
                            {displayName}
                        </span>
                    </nav>

                    {/* User Identity */}
                    <div className="flex items-start justify-between gap-6">
                        <div className="flex items-start gap-4">
                            <div
                                className="flex h-14 w-14 items-center justify-center rounded-xl shrink-0 text-white text-lg font-medium"
                                style={{ backgroundColor: userColor }}
                            >
                                {initials}
                            </div>
                            <div className="space-y-1">
                                <h1 className="text-2xl font-semibold tracking-tight">
                                    {displayName}
                                </h1>
                                <p className="text-sm text-muted-foreground flex items-center flex-wrap gap-x-0">
                                    {user.email && (
                                        <>
                                            <span>{user.email}</span>
                                            <span className="mx-2">·</span>
                                        </>
                                    )}
                                    {user.external_id && (
                                        <>
                                            <code className="text-xs bg-muted px-1.5 py-0.5 rounded">
                                                {user.external_id}
                                            </code>
                                            <span className="mx-2">·</span>
                                        </>
                                    )}
                                    <span>
                                        Created {formatDate(preferences, user.created_at, 'PP')}
                                    </span>
                                </p>
                            </div>
                        </div>

                        <DropdownMenu>
                            <DropdownMenuTrigger asChild>
                                <Button
                                    variant="ghost"
                                    size="icon"
                                    className="h-8 w-8"
                                >
                                    <MoreHorizontal className="h-4 w-4" />
                                </Button>
                            </DropdownMenuTrigger>
                            <DropdownMenuContent align="end">
                                <DropdownMenuItem
                                    className="text-destructive focus:text-destructive"
                                    onClick={() => setIsDeleteOpen(true)}
                                >
                                    <Trash2 className="h-4 w-4 mr-2" />
                                    {t('delete')}
                                </DropdownMenuItem>
                            </DropdownMenuContent>
                        </DropdownMenu>
                    </div>

                    {/* Navigation Tabs - Integrated with header */}
                    <nav className="flex gap-1 mt-6 -mb-px">
                        {tabs.map((tab) => {
                            const Icon = tab.icon
                            const isActive = activeTab === tab.key
                            return (
                                <NavLink
                                    key={tab.key}
                                    to={tab.to}
                                    end={tab.to === ''}
                                    className={cn(
                                        'flex items-center gap-2 px-4 py-2.5 text-sm font-medium rounded-t-lg border-b-2 transition-colors',
                                        isActive
                                            ? 'border-primary text-foreground bg-background'
                                            : 'border-transparent text-muted-foreground hover:text-foreground hover:bg-muted/50',
                                    )}
                                >
                                    <Icon className="h-4 w-4" />
                                    {tab.label}
                                </NavLink>
                            )
                        })}
                    </nav>
                </div>
            </div>

            {/* Content Area */}
            <div className="flex-1 p-6">
                <Outlet />
            </div>

            {/* Delete Confirmation Dialog */}
            <Dialog open={isDeleteOpen} onOpenChange={setIsDeleteOpen}>
                <DialogContent>
                    <DialogHeader>
                        <DialogTitle>{t('delete_user')}</DialogTitle>
                        <DialogDescription>
                            {t('delete_user_warning', 'Are you sure you want to delete this user? This action cannot be undone.')}
                        </DialogDescription>
                    </DialogHeader>
                    <div className="py-4">
                        <div className="flex items-center gap-3 p-3 rounded-lg bg-muted">
                            <div
                                className="flex h-10 w-10 items-center justify-center rounded-lg shrink-0 text-white text-sm font-medium"
                                style={{ backgroundColor: userColor }}
                            >
                                {initials}
                            </div>
                            <div>
                                <p className="font-medium">{displayName}</p>
                                <p className="text-sm text-muted-foreground">
                                    {user.email || user.external_id}
                                </p>
                            </div>
                        </div>
                    </div>
                    <DialogFooter>
                        <Button
                            variant="outline"
                            onClick={() => setIsDeleteOpen(false)}
                            disabled={isDeleting}
                        >
                            {t('cancel')}
                        </Button>
                        <Button
                            variant="destructive"
                            onClick={deleteUser}
                            disabled={isDeleting}
                        >
                            {isDeleting ? t('deleting') : t('delete_user')}
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>
        </div>
    )
}
