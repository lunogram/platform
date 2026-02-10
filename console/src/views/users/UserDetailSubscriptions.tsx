import { useCallback, useContext } from 'react'
import { useTranslation } from 'react-i18next'
import api from '../../api'
import { ProjectContext, UserContext } from '../../contexts'
import { useDebounceControl } from '../../hooks'
import type { SubscriptionParams, SubscriptionState, UserSubscription } from '../../types'
import { useSearchTableQueryState } from '../../ui/SearchTable'
import { snakeToTitle } from '../../utils'
import type { UUID } from '@/types/common'
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from '@/components/ui/table'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { Switch } from '@/components/ui/switch'
import {
    Pagination,
    PaginationContent,
    PaginationItem,
    PaginationPrevious,
    PaginationNext,
} from '@/components/ui/pagination'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Search } from 'lucide-react'

export default function UserDetailSubscriptions() {
    const { t } = useTranslation()
    const [project] = useContext(ProjectContext)
    const [user] = useContext(UserContext)

    const { results, params, setParams, reload } = useSearchTableQueryState(
        useCallback(async (params) => await api.users.subscriptions(project.id, user.id, params), [project, user])
    )

    const [search, setSearch] = useDebounceControl(params.q ?? '', q => setParams({ ...params, q }))

    const updateSubscription = async (subscription_id: UUID, state: SubscriptionState) => {
        if (!confirm(t('users_change_subscription_status'))) return
        await updateSubscriptions([{ subscription_id, state }])
    }

    const unsubscribeAll = async () => {
        if (!confirm(t('users_unsubscribe_all'))) return
        const subscriptions = results?.results.map(item => ({
            subscription_id: item.subscription_id,
            state: 'unsubscribed' as SubscriptionState,
        })) ?? []
        await updateSubscriptions(subscriptions)
    }

    const updateSubscriptions = async (subscriptions: SubscriptionParams[]) => {
        await api.users.updateSubscriptions(project.id, user.id, subscriptions)
        await reload()
    }

    return (
        <Card>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-4">
                <CardTitle>{t('subscriptions')}</CardTitle>
                <Button
                    size="sm"
                    variant="secondary"
                    onClick={unsubscribeAll}
                >
                    {t('unsubscribe_all')}
                </Button>
            </CardHeader>
            <CardContent className="space-y-4">
                <div className="relative w-full max-w-sm">
                    <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
                    <Input
                        type="search"
                        placeholder={t('search')}
                        value={search}
                        onChange={(e) => setSearch(e.target.value)}
                        className="pl-8"
                    />
                </div>

                <div className="rounded-md border">
                    <Table>
                        <TableHeader>
                            <TableRow>
                                <TableHead>{t('channel')}</TableHead>
                                <TableHead>{t('name')}</TableHead>
                                <TableHead>{t('subscribed')}</TableHead>
                            </TableRow>
                        </TableHeader>
                        <TableBody>
                            {!results ? (
                                Array.from({ length: 3 }).map((_, i) => (
                                    <TableRow key={i}>
                                        <TableCell colSpan={3}>
                                            <div className="h-4 w-3/4 animate-pulse rounded bg-muted" />
                                        </TableCell>
                                    </TableRow>
                                ))
                            ) : results.results.length === 0 ? (
                                <TableRow>
                                    <TableCell colSpan={3} className="text-center text-muted-foreground">
                                        {t('campaign.setup.channels.email.no_content_available')}
                                    </TableCell>
                                </TableRow>
                            ) : (
                                results.results.map((item: UserSubscription) => (
                                    <TableRow key={item.subscription_id}>
                                        <TableCell>{snakeToTitle(item.channel)}</TableCell>
                                        <TableCell>{item.name}</TableCell>
                                        <TableCell>
                                            <Switch
                                                checked={item.state === 'subscribed'}
                                                onCheckedChange={(checked) => updateSubscription(item.subscription_id, checked ? 'subscribed' : 'unsubscribed')}
                                            />
                                        </TableCell>
                                    </TableRow>
                                ))
                            )}
                        </TableBody>
                    </Table>
                </div>

                {results && (
                    <Pagination>
                        <PaginationContent>
                            {results.prevCursor && (
                                <PaginationItem>
                                    <PaginationPrevious
                                        onClick={() => setParams({ ...params, cursor: results.prevCursor!, page: 'prev' })}
                                    />
                                </PaginationItem>
                            )}
                            {results.nextCursor && (
                                <PaginationItem>
                                    <PaginationNext
                                        onClick={() => setParams({ ...params, cursor: results.nextCursor!, page: 'next' })}
                                    />
                                </PaginationItem>
                            )}
                        </PaginationContent>
                    </Pagination>
                )}
            </CardContent>
        </Card>
    )
}
