import { useCallback, useContext } from 'react'
import api from '../../api'
import { ProjectContext, UserContext } from '../../contexts'
import { useResolver } from '../../hooks'
import type { SubscriptionParams, SubscriptionState } from '../../types'
import { Button } from '@/components/ui/button'
import SwitchField from '../../ui/form/SwitchField'
import Heading from '../../ui/Heading'
import { SearchTable, useTableSearchParams } from '../../ui/SearchTable'
import { snakeToTitle } from '../../utils'
import { useTranslation } from 'react-i18next'
import type { UUID } from '@/types/common'

export default function UserDetailSubscriptions() {
    const { t } = useTranslation()
    const [project] = useContext(ProjectContext)
    const [user] = useContext(UserContext)
    const [params, setParams] = useTableSearchParams()

    const [search, , reload] = useResolver(useCallback(async () => await api.users.subscriptions(project.id, user.id, params), [project, user, params]))

    const updateSubscription = async (subscription_id: UUID, state: SubscriptionState) => {
        if (!confirm(t('users_change_subscription_status'))) return
        await updateSubscriptions([{ subscription_id, state }])
    }

    const unsubscribeAll = async () => {
        if (!confirm(t('users_unsubscribe_all'))) return
        const subscriptions = search?.results.map(item => ({
            subscription_id: item.subscription_id,
            state: 0 as SubscriptionState,
        })) ?? []
        await updateSubscriptions(subscriptions)
    }

    const updateSubscriptions = async (subscriptions: SubscriptionParams[]) => {
        await api.users.updateSubscriptions(project.id, user.id, subscriptions)
        await reload()
    }

    return <>
        <Heading
            size="h3"
            title={t('subscriptions')}
            actions={
                <Button
                    size="sm"
                    onClick={async () => await unsubscribeAll()}
                >{t('unsubscribe_all')}</Button>
            }
        />
        <SearchTable
            results={search}
            params={params}
            setParams={setParams}
            itemKey={({ item }) => item.subscription_id}
            columns={[
                {
                    key: 'channel',
                    title: t('channel'),
                    cell: ({ item: { channel } }) => snakeToTitle(channel),
                },
                { key: 'name', title: t('name') },
                {
                    key: 'state',
                    title: t('subscribed'),
                    cell: ({ item: { subscription_id, state } }) => {
                        return (
                            <SwitchField
                                name="state"
                                checked={state !== 0}
                                onChange={async (checked) => await updateSubscription(subscription_id, checked ? 1 : 0)}
                            />
                        )
                    },
                },
            ]}
        />
    </>
}
