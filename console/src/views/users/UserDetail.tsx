import { useContext, useState } from 'react'
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
import { NavLink } from 'react-router'
export default function UserDetail() {
    const { t } = useTranslation()
    const navigate = useNavigate()
    const location = useLocation()
    const [project] = useContext(ProjectContext)
    const [{ id, external_id, email, phone, timezone, full_name }] = useContext(UserContext)
    
    // Determine active tab from URL
    const pathParts = location.pathname.split('/')
    const lastPart = pathParts[pathParts.length - 1]
    const activeTab = ['events', 'subscriptions', 'journeys'].includes(lastPart) ? lastPart : 'details'
    const [isDeleteDialogOpen, setIsDeleteDialogOpen] = useState(false)
    const deleteUser = async () => {
        await api.users.delete(project.id, id)
        await navigate(`/projects/${project.id}/users`)
        setIsDeleteDialogOpen(false)
    }
    const userInfo = [
        { label: 'ID', value: external_id },
        { label: t('email'), value: email },
        { label: t('phone'), value: phone },
        { label: t('timezone'), value: timezone },
    ]
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
                                {userInfo.map(({ label, value }) => (
                                    value ? (
                                        <Badge key={label} variant="secondary" className="text-xs">
                                            {label}: {value}
                                        </Badge>
                                    ) : null
                                ))}
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
                    <Tabs value={activeTab} className="w-full">
                        <TabsList>
                            <TabsTrigger value="details" asChild>
                                <NavLink to="" end>{t('details')}</NavLink>
                            </TabsTrigger>
                            <TabsTrigger value="events" asChild>
                                <NavLink to="events">{t('events')}</NavLink>
                            </TabsTrigger>
                            <TabsTrigger value="subscriptions" asChild>
                                <NavLink to="subscriptions">{t('subscriptions')}</NavLink>
                            </TabsTrigger>
                            <TabsTrigger value="journeys" asChild>
                                <NavLink to="journeys">{t('journeys')}</NavLink>
                            </TabsTrigger>
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