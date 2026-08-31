import type { ReactNode } from "react"

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"

interface AuthCardProps {
    title: string
    description?: string
    children: ReactNode
    footer?: ReactNode
}

// AuthCard is the shell every unauthenticated view sits in. It exists so
// sign-in, registration, and the two password flows cannot drift apart
// visually — a login page that looks slightly different from one screen to the
// next is a page people hesitate to type a password into.
export default function AuthCard({ title, description, children, footer }: AuthCardProps) {
    return (
        <div className="min-h-screen flex items-center justify-center bg-muted/40 p-4">
            <Card className="w-full max-w-sm">
                <CardHeader className="space-y-1 text-center">
                    <CardTitle className="text-2xl font-bold">{title}</CardTitle>
                    {description && <CardDescription>{description}</CardDescription>}
                </CardHeader>
                <CardContent className="space-y-4">{children}</CardContent>
                {footer && (
                    <div className="px-6 pb-6 text-center text-sm text-muted-foreground">
                        {footer}
                    </div>
                )}
            </Card>
        </div>
    )
}
