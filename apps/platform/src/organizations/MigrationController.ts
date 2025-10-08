import Router from '@koa/router'
import { JwtAdmin } from '../auth/AuthMiddleware'
import MigrateJob from './MigrateJob'

const router = new Router<{
    admin: JwtAdmin
}>({
    prefix: '/migrate',
})

router.post('/events', async ctx => {
    await MigrateJob.from({ type: 'events', since: ctx.query.since as string })
        .jobId('migrate_events')
        .queue()
    ctx.status = 204
})

router.post('/users', async ctx => {
    await MigrateJob.from({
        type: 'users',
        since: ctx.query.since as string,
        user_id: ctx.query.user_id
            ? parseInt(ctx.query.user_id as string, 10)
            : undefined,
    })
        .jobId('migrate_users')
        .queue()
    ctx.status = 204
})

router.post('/lists', async ctx => {
    await MigrateJob.from({ type: 'lists', since: ctx.query.since as string })
        .jobId('migrate_lists')
        .queue()
    ctx.status = 204
})

export default router
