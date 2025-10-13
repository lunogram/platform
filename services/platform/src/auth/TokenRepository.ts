import { addSeconds } from 'date-fns'
import { Context } from 'koa'
import jwt from 'jsonwebtoken'
import { AccessToken } from './AccessToken'
import App from '../app'
import Admin from './Admin'

export interface OAuthResponse {
    access_token: string
    expires_at: Date
}

export async function cleanupExpiredRevokedTokens(until: Date) {
    await AccessToken.delete(qb => qb.where('expires_at', '<=', until))
}

export const generateAccessToken = async ({ id }: Admin, ctx?: Context) => {
    const expires_at = addSeconds(Date.now(), App.main.env.auth.tokenLife)
    const token = jwt.sign({
        sub: id,
        iss: App.main.env.baseUrl,
        exp: Math.floor(expires_at.getTime() / 1000),
    }, App.main.env.secret)

    await AccessToken.insert({
        admin_id: id,
        expires_at,
        token,
        revoked: false,
        ip: ctx?.request.ip ?? '',
        user_agent: ctx?.request.headers['user-agent'] || '',
    })

    return {
        access_token: token,
        expires_at,
    }
}

export const getCookiesOAuthToken = (ctx: Context) => {
    const cookie = ctx.cookies.get('oauth')
    if (cookie) {
        return JSON.parse(cookie) as OAuthResponse
    }
}

export const setCookiesOauthToken = (ctx: Context, oauth: OAuthResponse): OAuthResponse => {
    ctx.cookies.set('oauth', JSON.stringify(oauth), {
        secure: ctx.request.secure,
        httpOnly: true,
        expires: oauth.expires_at,
    })

    return oauth
}
