import { useSearchParams } from "react-router"
import { SignUp } from "@clerk/clerk-react"

export default function Register() {
    const [searchParams] = useSearchParams()
    const redirect = searchParams.get("r") ?? "/"

    return (
        <div className="min-h-screen flex flex-col items-center justify-center bg-muted/40 p-4 gap-4">
            <SignUp forceRedirectUrl={`/login/clerk/callback?r=${redirect}`} />
        </div>
    )
}
