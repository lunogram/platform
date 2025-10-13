import { Context } from 'koa'
import AuthProvider from './AuthProvider'
import OpenIDProvider, { OpenIDConfig } from './OpenIDAuthProvider'
import GoogleProvider, { GoogleConfig } from './GoogleAuthProvider'
import CloudProvider, { CloudConfig } from './CloudAuthProvider'
import SAMLProvider, { SAMLConfig } from './SAMLAuthProvider'
import { DriverConfig } from '../config/env'
import BasicAuthProvider, { BasicAuthConfig } from './BasicAuthProvider'
import Organization from '../organizations/Organization'
import App from '../app'
import EmailAuthProvider, { EmailAuthConfig } from './EmailAuthProvider'

export type AuthProviderName = 'basic' | 'email' | 'saml' | 'openid' | 'google' | 'cloud'

export type AuthProviderConfig = BasicAuthConfig | EmailAuthConfig | SAMLConfig | OpenIDConfig | GoogleConfig | CloudConfig

export interface AuthConfig {
    driver: AuthProviderName[]
    tokenLife: number
    jwt: JWTConfig
    basic: BasicAuthConfig
    email: EmailAuthConfig
    saml: SAMLConfig
    openid: OpenIDConfig
    google: GoogleConfig
    cloud: CloudConfig
}

export interface JWTConfig {
    jwksUrl?: string
}

export { BasicAuthConfig, SAMLConfig, OpenIDConfig }

export interface AuthTypeConfig extends DriverConfig {
    driver: AuthProviderName
    name?: string
}

interface AuthMethod {
    driver: AuthProviderName
    name: string
    publicConfig?: { [key: string]: string }
}

export const initProvider = (config?: AuthProviderConfig): AuthProvider => {
    switch (config?.driver) {
    case 'basic':
        return new BasicAuthProvider(config)
    case 'email':
        return new EmailAuthProvider(config)
    case 'saml':
        return new SAMLProvider(config)
    case 'openid':
        return new OpenIDProvider(config)
    case 'google':
        return new GoogleProvider(config)
    case 'cloud':
        return new CloudProvider(config)
    default:
        throw new Error('A valid auth driver must be set!')
    }
}

export const authMethods = async (): Promise<AuthMethod[]> => {
    return mapMethods(App.main.env.auth)
}

export const checkAuth = (organization?: Organization): boolean => {
    return organization != null && organization.auth != null
}

export const startAuth = async (ctx: Context): Promise<void> => {
    const provider = await loadProvider(ctx)
    return await provider.start(ctx)
}

export const validateAuth = async (ctx: Context): Promise<void> => {
    const provider = await loadProvider(ctx)
    return await provider.validate(ctx)
}

export const authWebhook = async (ctx: Context): Promise<void> => {
    const provider = await loadProvider(ctx)
    if (!provider.webhook) {
        return ctx.throw(404)
    }

    return await provider.webhook(ctx)
}

const loadProvider = async (ctx: Context): Promise<AuthProvider> => {
    const driver = ctx.params.driver as AuthProviderName
    return initProvider(App.main.env.auth[driver])
}

const mapMethods = (config: AuthConfig): AuthMethod[] => {
    const drivers = config.driver
    return drivers.map((driver) => mapMethod(config[driver]))
}

const mapMethod = ({ driver, name }: AuthTypeConfig): AuthMethod => ({
    driver,
    name: name ?? `Continue with ${driver}`,
})
