import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { CopyIcon } from "@/components/icons"
import type { ReactNode } from "react"
import Heading from "@/components/heading"

interface CodeExampleProps {
    code: string
    title?: ReactNode
    description?: ReactNode
}

export default function CodeExample({ code, description, title }: CodeExampleProps) {
    const handleCopy = async (value: string) => {
        await navigator.clipboard.writeText(value)
        toast.success("Copied code sample")
    }

    return (
        <>
            {Boolean(title ?? description) && (
                <Heading title={title} size="h4">
                    {description}
                </Heading>
            )}
            <div className="relative">
                <pre className="rounded-md bg-muted break-keep overflow-hidden p-5 m-0">
                    <code>{code}</code>
                </pre>
                <div className="absolute right-2.5 top-2.5">
                    <Button
                        variant="secondary"
                        size="sm"
                        onClick={async () => await handleCopy(code)}
                    >
                        <CopyIcon />
                    </Button>
                </div>
            </div>
        </>
    )
}
