import React from "react"
import { Card } from "@/components/ui/card"
import { cn } from "@/utils"

interface ChoiceCardProps {
    title: string
    description: string
    icon: React.ReactNode
    onClick: () => void
    variant?: "default" | "dashed"
    className?: string
}

export const ChoiceCard = ({
    title,
    description,
    icon,
    onClick,
    variant = "default",
    className,
}: ChoiceCardProps) => (
    <Card
        role="button"
        onClick={onClick}
        className={cn(
            "group flex cursor-pointer flex-col items-center justify-center p-8 text-center transition-colors hover:border-primary hover:bg-accent aspect-4/3",
            variant === "dashed" && "border-dashed",
            className,
        )}
    >
        <div className="mb-4 rounded-lg bg-muted p-4 text-muted-foreground group-hover:bg-primary group-hover:text-primary-foreground transition-colors">
            {React.cloneElement(
                icon as React.ReactElement,
                {
                    className: "h-6 w-6",
                } as React.SVGProps<SVGSVGElement>,
            )}
        </div>
        <p className="text-base font-medium">{title}</p>
        <p className="text-sm text-muted-foreground mt-1.5">{description}</p>
    </Card>
)
