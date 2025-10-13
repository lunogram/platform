import jwt, { Jwt, JwtHeader, JwtPayload, SigningKeyCallback } from 'jsonwebtoken'
import jwks from 'jwks-rsa'
import { Context } from 'koa'
import App from '../app'
import { RequestError } from '../core/errors'
import Project, { ProjectRole } from '../projects/Project'
import { ProjectApiKey } from '../projects/ProjectApiKey'
import { getProjectApiKey } from '../projects/ProjectService'
import AuthError from './AuthError'
import { getCookiesOAuthToken } from './TokenRepository'
import { OrganizationRole } from '../organizations/Organization'
import { UUID } from 'node:crypto'
import { getAdminByExternalId, getAdminById } from './AdminRepository'
import Admin from './Admin'

export interface JwtAdmin {
    id: UUID
    organization_id: UUID
    role: OrganizationRole
}

export interface State {
    app: App
}

type AuthScope = 'admin' | 'public' | 'secret'
export interface AuthState {
    scope: AuthScope
    admin?: JwtAdmin
    key?: ProjectApiKey
}

export interface ProjectState extends AuthState {
    project: Project
    projectRole: ProjectRole
}

export function retrieveAuthToken(ctx: Context): string | undefined {
    const tokenSameOrigin = ctx.cookies.get('__session')
    const tokenCrossOrigin = ctx.headers.authorization
    const tokenOAuth = getCookiesOAuthToken(ctx)?.access_token
    return tokenOAuth || tokenSameOrigin || tokenCrossOrigin
}

const parseAuth = async (ctx: Context) => {
    const token = retrieveAuthToken(ctx)

    if (!token) {
        throw new RequestError(AuthError.AuthorizationError)
    }

    if (isPublicKey(token)) {
        return createPublicScope(parseBearer(token))
    }

    if (isSecretKey(token)) {
        return createSecretScope(parseBearer(token))
    }

    return await createAdminScope(token)
}

function parseBearer(token: string) {
    return token.replace('Bearer ', '')
}

function isPublicKey(token: string): boolean {
    return token.startsWith('Bearer pk_')
}

function isSecretKey(token: string): boolean {
    return token.startsWith('Bearer sk_')
}

async function createPublicScope(token: string) {
    return {
        scope: 'public' as const,
        key: await getProjectApiKey(token),
    }
}

async function createSecretScope(token: string) {
    return {
        scope: 'secret' as const,
        key: await getProjectApiKey(token),
    }
}

async function createAdminScope(token: string) {
    const payload = await jwtVerify(token)
    if (!payload || !payload.sub) {
        throw new RequestError(AuthError.InvalidToken)
    }

    let admin: Admin | undefined
    if (payload.iss && payload.iss !== App.main.env.baseUrl) {
        admin = await getAdminByExternalId(payload.sub)
    }

    if (!admin) {
        admin = await getAdminById(payload.sub as UUID)
    }

    if (!admin) {
        throw new RequestError(AuthError.InvalidToken)
    }

    return {
        scope: 'admin' as const,
        admin: {
            id: admin.id,
            organization_id: admin.organization_id,
            role: admin.role,
        },
    }
}

export async function authMiddleware(ctx: Context, next: () => void) {
    try {
        const state = await parseAuth(ctx)
        ctx.state = { ...ctx.state, ...state }
    } catch (error) {
        throw new RequestError(AuthError.AuthorizationError)
    }
    return next()
}

export const scopeMiddleware = (scope: string | string[]) => {
    const scopes = Array.isArray(scope) ? scope : [scope]
    return async function authMiddleware(ctx: Context, next: () => void) {
        if (!scopes.includes(ctx.state.scope)) {
            throw new RequestError(AuthError.AccessDenied)
        }
        return next()
    }
}

let jwksClient: jwks.JwksClient | undefined

export const jwtVerify = async (token: string): Promise<JwtPayload> => {
    const secret = App.main.env.secret

    if (App.main.env.auth.jwt.jwksUrl) {
        jwksClient = jwks({
            jwksUri: App.main.env.auth.jwt.jwksUrl,
            cache: true,
            rateLimit: true,
        })
    }

    if (jwksClient) {
        const options: jwt.VerifyOptions = { algorithms: ['RS256'] }

        const getKey = (header: JwtHeader, callback: SigningKeyCallback) => {
            if (!header.kid) {
                return callback(new Error('Missing KID in token header'))
            }
            jwksClient!.getSigningKey(header.kid, (err, key) => {
                if (err) return callback(err)
                if (!key) return callback(new Error('No signing key found'))
                const signingKey = key.getPublicKey()
                callback(null, signingKey)
            })
        }

        // Verify the token using the dynamic JWKS public key
        return new Promise((resolve, reject) => {
            jwt.verify(token, getKey, options, (err, decoded) => {
                if (err) {
                    return reject(new RequestError(AuthError.InvalidToken))
                }
                if (!validJWTToken(decoded)) {
                    return reject(new RequestError(AuthError.InvalidToken))
                }
                resolve(decoded as JwtPayload)
            })
        })
    }

    return new Promise((resolve, reject) => {
        jwt.verify(token, secret, (err, decoded) => {
            if (err) {
                return reject(new RequestError(AuthError.InvalidToken))
            }
            if (!validJWTToken(decoded)) {
                return reject(new RequestError(AuthError.InvalidToken))
            }
            resolve(decoded as JwtPayload)
        })
    })
}

function validJWTToken(decoded: string | JwtPayload | Jwt | undefined): boolean {
    if (decoded === undefined || typeof decoded === 'string') {
        return false
    }

    let payload = decoded as JwtPayload
    if (decoded.payload) {
        payload = decoded.payload
    }

    const currentTime = Math.floor(Date.now() / 1000)
    if (payload.exp && payload.exp < currentTime) {
        return false
    }

    if (payload.nbf && payload.nbf > currentTime) {
        return false
    }

    // TODO: validate the token's authorized party (azp) claim

    return true
}
