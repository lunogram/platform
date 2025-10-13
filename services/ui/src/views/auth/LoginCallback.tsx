import { useParams, useSearchParams } from 'react-router'
import { useEffect } from 'react'
import api from '../../api'

import './Auth.css'

export default function LoginCallback() {
    const { driver } = useParams() as { driver: string }
    const [searchParams] = useSearchParams()
    const redirect = searchParams.get('r') ?? '/'

    const handleAuth = async () => {
        switch (driver) {
        case 'cloud':
            await api.auth.cloudAuth(redirect)
        }

        window.location.href = redirect
    }

    useEffect(() => {
        handleAuth().catch((err) => {
            console.error('Authentication error', err)
        })
    }, [driver, redirect])

    // TODO: handle callback error
    return <></>
}
