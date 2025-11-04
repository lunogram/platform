import { useState } from "react";
import { Input } from "@/components/ui/input";
import {
    Select,
    SelectTrigger,
    SelectValue,
    SelectContent,
    SelectItem,
} from "@/components/ui/select";
import { cn } from "@/utils";

interface InputWithUnitProps {
    value?: number;
    unit?: "px" | "%" | "rem";
    onChange?: (value: number) => void;
    onUnitChange?: (unit: "px" | "%" | "rem") => void;
    className?: string;
}

export function InputWithUnit({
    value = 0,
    unit = "px",
    onChange,
    onUnitChange,
    className,
}: InputWithUnitProps) {
    const [localValue, setLocalValue] = useState<number>(value);
    const [localUnit, setLocalUnit] = useState<"px" | "%" | "rem">(unit);

    const handleValueChange = (e: React.ChangeEvent<HTMLInputElement>) => {
        const num = Number(e.target.value);
        setLocalValue(num);
        onChange?.(num);
    };

    const handleUnitChange = (u: "px" | "%" | "rem") => {
        setLocalUnit(u);
        onUnitChange?.(u);
    };

    return (
        <div
            className={cn(
                "flex items-center w-full rounded-md border border-input bg-background shadow-sm focus-within:ring-2 focus-within:ring-ring focus-within:ring-offset-1",
                className
            )}
        >
            <Input
                type="number"
                value={localValue}
                onChange={handleValueChange}
                className="border-0 focus-visible:ring-0 focus-visible:ring-offset-0 rounded-r-none flex-1"
            />

            <Select value={localUnit} onValueChange={handleUnitChange}>
                <SelectTrigger className="w-16 rounded-l-none border-0 border-l border-border focus:ring-0 focus:ring-offset-0">
                    <SelectValue />
                </SelectTrigger>
                <SelectContent>
                    <SelectItem value="px">px</SelectItem>
                    <SelectItem value="%">%</SelectItem>
                    <SelectItem value="rem">rem</SelectItem>
                </SelectContent>
            </Select>
        </div>
    );
}
