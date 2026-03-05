import { Button } from "@/components/ui/button"
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible"
import { ChevronsUpDown } from "lucide-react"

interface CollapsibleFieldProps {
    icon: React.ReactNode
    title: string
    children: React.ReactNode
}

export default function CollapsibleField({ icon, title, children }: CollapsibleFieldProps) {
    return (
        <Collapsible className="w-full">
            <CollapsibleTrigger className="flex w-full justify-between items-center pb-1 cursor-pointer">
                <div className="flex items-center ml-1 gap-1 text-[color:var(--puck-color-grey-04)] font-semibold text-[length:var(--puck-font-size-xxs)]">
                    <div className="[&>svg]:w-4 [&>svg]:h-4 text-[color:var(--puck-color-grey-07)]">
                        {icon}
                    </div>
                    {title}
                </div>
                <Button variant="ghost" size="sm">
                    <ChevronsUpDown strokeWidth={2} />
                </Button>
            </CollapsibleTrigger>
            <CollapsibleContent>{children}</CollapsibleContent>
        </Collapsible>
    )
}
