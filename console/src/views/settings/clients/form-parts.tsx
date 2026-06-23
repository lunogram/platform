import { Label } from "@/components/ui/label"

// Field is a single labelled form control with an optional helper line beneath.
export function Field({
    label,
    hint,
    children,
}: {
    label: string
    hint?: string
    children: React.ReactNode
}) {
    return (
        <div className="grid gap-1.5">
            <Label>{label}</Label>
            {children}
            {hint && <p className="text-xs text-ink-soft">{hint}</p>}
        </div>
    )
}
