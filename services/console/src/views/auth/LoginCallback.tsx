import { useParams, useSearchParams } from 'react-router'
import { useEffect } from 'react'
import { useClerk } from '@clerk/clerk-react'
import api from '../../api'

import './Auth.css'

export default function LoginCallback() {
    const { session } = useClerk()
    const { driver } = useParams() as { driver: string }
    const [searchParams] = useSearchParams()
    const redirect = searchParams.get('r') ?? '/'

    const handleAuth = async () => {
        switch (driver) {
        case 'cloud':{
            if (!session) return

            await session.getToken()
            await api.auth.cloudAuth(redirect)
            break
        }
        }

        window.location.href = redirect
    }

    useEffect(() => {
        handleAuth().catch((err) => {
            console.error('Authentication error', err)
        })
    }, [driver, redirect, session])

    // TODO: handle callback error
    return <></>
}
