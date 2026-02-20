import { useContext, useEffect, useState } from 'react'
import { ProjectContext, UserContext } from '../../contexts'
import { Outlet, useNavigate, useLocation } from 'react-router'
import { Button } from '@/components/ui/button'
import {
    Card,
    CardContent,
    CardHeader,
    CardTitle,
    CardDescription,
} from '@/components/ui/card'
import {
    Tabs,
    TabsList,
    TabsTrigger,
} from '@/components/ui/tabs'
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from '@/components/ui/dialog'
import { Separator } from '@/components/ui/separator'
import { Badge } from '@/components/ui/badge'
import { TrashIcon } from '../../components/icons'
import api from '../../api'
import { useTranslation } from 'react-i18next'

export default function UserDetail() {
    const { t } = useTranslation()
    const navigate = useNavigate()
    const location = useLocation()
    const [project] = useContext(ProjectContext)
    const [{ id, external_id, email, phone, timezone, full_name }] = useContext(UserContext)
    
    type TabValue = 'details' | 'events' | 'subscriptions' | 'journeys'
    const [activeTab, setActiveTab] = useState<TabValue>(() => {
        const pathParts = location.pathname.split('/')
        const lastPart = pathParts[pathParts.length - 1]
        return ['events', 'subscriptions', 'journeys'].includes(lastPart) ? lastPart as TabValue : 'details'
    })

    useEffect(() => {
        const newPath = activeTab === 'details' 
            ? location.pathname.replace(/\/(events|subscriptions|journeys)$/, '')
            : location.pathname.replace(/\/(events|subscriptions|journeys)$/, '') + `/${activeTab}`
        if (newPath !== location.pathname) {
            navigate(newPath, { replace: true })
        }
    }, [activeTab, location.pathname, navigate])

    const [isDeleteDialogOpen, setIsDeleteDialogOpen] = useState(false)
    const deleteUser = async () => {
        await api.users.delete(project.id, id)
        await navigate(`/projects/${project.id}/users`)
        setIsDeleteDialogOpen(false)
    }
    return (
        <div className="py-8 px-8 space-y-6">
            <Card>
                <CardHeader className="flex flex-row items-start justify-between space-y-0">
                    <div className="space-y-1.5">
                        <CardTitle className="text-2xl font-bold tracking-tight">
                            {full_name ?? (email ?? 'No email')}
                        </CardTitle>
                        <CardDescription>
                            <div className="flex flex-wrap gap-2 mt-2">
                                {external_id && (
                                    <Badge variant="secondary" className="text-xs">
                                        {t('external_id')}: {external_id}
                                    </Badge>
                                )}
                                {email && (
                                    <Badge variant="secondary" className="text-xs">
                                        {t('email')}: {email}
                                    </Badge>
                                )}
                                {phone && (
                                    <Badge variant="secondary" className="text-xs">
                                        {t('phone')}: {phone}
                                    </Badge>
                                )}
                                {timezone && (
                                    <Badge variant="secondary" className="text-xs">
                                        {t('timezone')}: {timezone}
                                    </Badge>
                                )}
                            </div>
                        </CardDescription>
                    </div>
                    <Button
                        onClick={() => setIsDeleteDialogOpen(true)}
                        variant="destructive"
                        size="sm"
                    >
                        <TrashIcon />
                        <span className="ml-2">{t('delete_user')}</span>
                    </Button>
                </CardHeader>
                <Separator />
                <CardContent className="pt-6">
                    <Tabs value={activeTab} onValueChange={(value) => setActiveTab(value as TabValue)} className="w-full">
                        <TabsList>
                            <TabsTrigger value="details">{t('details')}</TabsTrigger>
                            <TabsTrigger value="events">{t('events')}</TabsTrigger>
                            <TabsTrigger value="subscriptions">{t('subscriptions')}</TabsTrigger>
                            <TabsTrigger value="journeys">{t('journeys')}</TabsTrigger>
                        </TabsList>
                    </Tabs>
                    <div className="mt-6">
                        <Outlet />
                    </div>
                </CardContent>
            </Card>
            <Dialog open={isDeleteDialogOpen} onOpenChange={setIsDeleteDialogOpen}>
                <DialogContent>
                    <DialogHeader>
                        <DialogTitle>{t('delete_user')}</DialogTitle>
                        <DialogDescription>
                            {t('delete_user_confirmation')}
                        </DialogDescription>
                    </DialogHeader>
                    <DialogFooter>
                        <Button
                            variant="outline"
                            onClick={() => setIsDeleteDialogOpen(false)}
                        >
                            {t('cancel')}
                        </Button>
                        <Button
                            variant="destructive"
                            onClick={deleteUser}
                        >
                            {t('delete')}
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>
        </div>
    )
}