import { Label } from "@/components/ui/label"

// Field is a single labelled form control with an optional helper line beneath.
// Pass `htmlFor` (and give the control a matching id) to associate the label with
// its control, so it is reachable by its accessible name.
export function Field({
    label,
    hint,
    htmlFor,
    children,
}: {
    label: string
    hint?: string
    htmlFor?: string
    children: React.ReactNode
}) {
    return (
        <div className="grid gap-1.5">
            <Label htmlFor={htmlFor}>{label}</Label>
            {children}
            {hint && <p className="text-xs text-ink-soft">{hint}</p>}
        </div>
    )
}
