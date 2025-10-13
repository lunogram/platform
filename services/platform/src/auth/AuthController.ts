import Router from '@koa/router'
import { getOrganizationByEmail } from '../organizations/OrganizationService'
import Organization from '../organizations/Organization'
import { authMethods, authWebhook, checkAuth, startAuth, validateAuth } from './Auth'

const router = new Router<{
    organization?: Organization
}>({
    prefix: '/auth',
})

router.get('/methods', async ctx => {
    ctx.body = await authMethods()
})

router.post('/check', async ctx => {
    const email = ctx.query.email || ctx.request.body.email
    const organization = await getOrganizationByEmail(email)
    ctx.body = checkAuth(organization)
})

router.get('/login/:driver', async ctx => {
    ctx.status = 204
    await startAuth(ctx)
})

router.post('/login/:driver', async ctx => {
    ctx.status = 204
    await startAuth(ctx)
})

router.get('/login/:driver/callback', async ctx => {
    ctx.status = 204
    await validateAuth(ctx)
})

router.post('/login/:driver/callback', async ctx => {
    ctx.status = 204
    await validateAuth(ctx)
})

router.post('/login/:driver/webhook', async ctx => {
    await authWebhook(ctx)
})

export default router
