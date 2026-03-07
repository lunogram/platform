import { isRouteErrorResponse, Navigate, useNavigate, useRouteError } from "react-router"
import { Alert, AlertTitle, AlertDescription } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { logout } from "../utils"
import { useClerk } from "@clerk/clerk-react"

import "./ErrorPage.css"

const ErrorAlert = ({
    variant,
    title,
    actions,
    children,
}: {
    variant?: "default" | "destructive"
    title: string
    actions?: React.ReactNode
    children?: React.ReactNode
}) => {
    return (
        <section className="error-page">
            <Alert variant={variant}>
                <AlertTitle>{title}</AlertTitle>
                <AlertDescription>{children}</AlertDescription>
                {actions && <div className="mt-2 flex gap-2">{actions}</div>}
            </Alert>
        </section>
    )
}

export default function ErrorPage({ status = 500 }: { status?: number }) {
    const error = useRouteError() as any
    const navigate = useNavigate()

    console.error(error)

    let message = ""
    if (isRouteErrorResponse(error)) {
        status = error.status
        message = error.data + ""
    }
    if (error?.response) {
        status = error.response.status
        message = error.response.data + ""
    }

    if (status === 401) {
        // in case the data router didn't catch this already
        return <Navigate to="/login" />
    }

    if (status === 403) {
        return <AccessDenied />
    }

    if (status === 404) {
        sessionStorage.clear()

        return (
            <ErrorAlert
                variant="default"
                title="Looks Like You're Lost!"
                actions={
                    <Button
                        onClick={async () => {
                            await navigate("/")
                        }}
                    >
                        Go Back
                    </Button>
                }
            >
                The page or resource you are looking for does not exist or has been moved.
            </ErrorAlert>
        )
    }

    return (
        <ErrorAlert variant="destructive" title={`Error [${status.toString()}]`}>
            {message}
        </ErrorAlert>
    )
}

export function AccessDenied() {
    const { signOut } = useClerk()

    return (
        <ErrorAlert
            variant="default"
            title="Access Denied"
            actions={
                <>
                    <Button
                        onClick={async () => {
                            await logout(signOut)
                        }}
                    >
                        Logout
                    </Button>
                    <Button
                        onClick={() => {
                            window.location.href = "/"
                        }}
                    >
                        Back
                    </Button>
                </>
            }
        >
            Additional permission is required in order to access this section. Please reach out to
            your administrator.
        </ErrorAlert>
    )
}
