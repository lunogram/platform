import { Context } from 'koa'
import { AuthTypeConfig } from './Auth'
import AuthProvider from './AuthProvider'
import App from '../app'
import { firstQueryParam } from '../utilities'
import { RequestError } from '../core/errors'
import AuthError from './AuthError'
import { sign } from 'jsonwebtoken'
import { addSeconds } from 'date-fns'
import SMTPEmailProvider, { SMTPDataParams } from '../providers/email/SMPTEmailProvider'
import { jwtVerify } from './AuthMiddleware'

export interface EmailAuthConfig extends AuthTypeConfig, SMTPDataParams {
    driver: 'email'
    from: string
}

export default class EmailAuthProvider extends AuthProvider {

    private config: EmailAuthConfig
    private provider: SMTPEmailProvider
    constructor(config: EmailAuthConfig) {
        super()
        this.config = config
        this.provider = SMTPEmailProvider.fromJson(config)
        this.provider.boot()
    }

    async start(ctx: Context) {

        const redirect = firstQueryParam(ctx.request.query.r)
        const email = ctx.request.body.email

        if (email) {
            await this.send(email, redirect)
        } else {
            ctx.redirect(new URL(`/login/email?r=${redirect}`, App.main.env.baseUrl).toString())
        }
    }

    async validate(ctx: Context) {
        const token = firstQueryParam(ctx.request.query.token)
        if (!token) throw new RequestError(AuthError.MissingCredentials)

        const { email } = await jwtVerify(token) as { email: string }
        await this.login({
            email,
            first_name: 'Admin',
        }, ctx)
    }

    async send(email: string, redirect: string | undefined) {

        // JWT with a fifteen minute expiration
        const expiresAt = addSeconds(Date.now(), 15 * 60)
        const token = sign({
            email,
            exp: Math.floor(expiresAt.getTime() / 1000),
        }, App.main.env.secret)

        // Generate the link
        const link = new URL('/auth/login/email/callback', App.main.env.apiBaseUrl)
        link.searchParams.set('token', token)
        if (redirect) link.searchParams.set('r', redirect)

        // Send the message
        await this.provider.send({
            to: email,
            from: this.config.from,
            subject: 'Login to Lunogram',
            html: this.generateMessage(link.toString()),
            text: `Click the link below to login to Lunogram: ${link}`,
        })
    }

    generateMessage(link: string) {
        return `
            <html>
                <head>
                    <style type="text/css">
                        body { font-family: "Helvetica Neue", "Helvetica", "sans-serif"; background: #F5F5F5 }
                        body, p, .button { font-size: 16px; }
                        h2 {
                            font-weight: 600;
                            font-size: 24px;
                            margin-bottom: 0;
                        }
                        .button {
                            border-radius: 10px;
                            background: #000;
                            padding: 10px 20px;
                            color: #FFF;
                            display: inline-block;
                            text-decoration: none;
                        }
                    </style>
                </head>
                <body>
                    <center>
                        <table width="100%" border="0" cellpadding="0" cellspacing="0" border="0">
                            <tbody>
                                <tr>
                                    <td align="center" valign="top" style="padding: 20px 0px">
                                        <a href="https://lunogram.com">
                                            <img src="https://lunogram.com/images/lunogram.svg" alt="Logo">
                                        </a>
                                    </td>
                                </tr>
                            </tbody>
                        </table>
                        <table bgcolor="#FFF" width="90%" cellspacing="0" align="center" border="0" style="border-radius: 15px; max-width: 600px">
                            <tbody>
                                <tr>
                                    <td align="left" style="padding: 25px 30px;">
                                        <h2>Hello!</h2>
                                        <p>You asked us to send you a magic link to get you signed in to Lunogram! Hit the button below to continue.</p>
                                        <a class="button" href="${link}">Sign in to Lunogram</a>
                                        <p>Note: The link expires after 15 minutes.</p>
                                    </td>
                                </tr>
                            </tbody>
                        </table>
                    </center>
                </body>
            </html>
        `
    }
}
