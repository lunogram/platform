import React from "react";
import { Card } from "@/components/ui/card";
import { cn } from "@/utils";

interface ChoiceCardProps {
  title: string;
  description: string;
  icon: React.ReactNode;
  onClick: () => void;
  variant?: "default" | "dashed";
  className?: string;
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
      "group relative flex cursor-pointer flex-col items-center justify-center p-6 text-center transition-all hover:border-primary hover:bg-accent",
      variant === "dashed" && "border-dashed bg-muted/50",
      className,
    )}
  >
    <div className="mb-4 rounded-full bg-secondary p-4 text-secondary-foreground group-hover:bg-primary group-hover:text-primary-foreground">
      {React.cloneElement(
        icon as React.ReactElement,
        { size: 28 } as React.SVGProps<SVGSVGElement>,
      )}
    </div>
    <h3 className="font-bold tracking-tight">{title}</h3>
    <p className="text-sm text-muted-foreground">{description}</p>
  </Card>
);
