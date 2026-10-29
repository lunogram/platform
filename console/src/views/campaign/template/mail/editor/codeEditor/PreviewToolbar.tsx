import { Crosshair, Monitor, Rocket, Smartphone, Tablet } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import { isEnterprise } from "@/config/enterprise"
import type { EditorMode } from "./hooks/useEditorMode"
import type { Viewport } from "./types"

export function SendTestButton({ sending, onClick }: { sending: boolean; onClick: () => void }) {
    return (
        <Button
            variant="default"
            size="sm"
            className="h-8 gap-1.5 px-2.5"
            onClick={onClick}
            disabled={sending}
        >
            <Rocket className="h-3.5 w-3.5" />
            <span className="text-xs">Send test</span>
        </Button>
    )
}

export function PreviewToolbar({
    viewport,
    setViewport,
    selectorActive,
    setSelectorActive,
    editorMode,
    sending,
    onSendTest,
}: {
    viewport: Viewport
    setViewport: (v: Viewport) => void
    selectorActive: boolean
    setSelectorActive: (v: boolean) => void
    editorMode: EditorMode
    sending: boolean
    onSendTest: () => void
}) {
    return (
        <div className="flex items-center gap-1 pr-3">
            {isEnterprise && editorMode === "builder" && (
                <>
                    <Tooltip>
                        <TooltipTrigger asChild>
                            <Button
                                variant={selectorActive ? "secondary" : "ghost"}
                                size="sm"
                                className={`h-8 w-8 p-0 ${selectorActive ? "bg-blue-500/10 text-blue-600 hover:bg-blue-500/20 dark:text-blue-400" : ""}`}
                                onClick={() => setSelectorActive(!selectorActive)}
                            >
                                <Crosshair className="h-4 w-4" />
                            </Button>
                        </TooltipTrigger>
                        <TooltipContent>
                            {selectorActive ? "Cancel selection" : "Select element"}
                        </TooltipContent>
                    </Tooltip>
                    <div className="w-px h-4 bg-border mx-1" />
                </>
            )}
            <Tooltip>
                <TooltipTrigger asChild>
                    <Button
                        variant={viewport === "mobile" ? "secondary" : "ghost"}
                        size="sm"
                        className="h-8 w-8 p-0"
                        onClick={() => setViewport("mobile")}
                    >
                        <Smartphone className="h-4 w-4" />
                    </Button>
                </TooltipTrigger>
                <TooltipContent>Mobile (375px)</TooltipContent>
            </Tooltip>
            <Tooltip>
                <TooltipTrigger asChild>
                    <Button
                        variant={viewport === "tablet" ? "secondary" : "ghost"}
                        size="sm"
                        className="h-8 w-8 p-0"
                        onClick={() => setViewport("tablet")}
                    >
                        <Tablet className="h-4 w-4" />
                    </Button>
                </TooltipTrigger>
                <TooltipContent>Tablet (768px)</TooltipContent>
            </Tooltip>
            <Tooltip>
                <TooltipTrigger asChild>
                    <Button
                        variant={viewport === "desktop" ? "secondary" : "ghost"}
                        size="sm"
                        className="h-8 w-8 p-0"
                        onClick={() => setViewport("desktop")}
                    >
                        <Monitor className="h-4 w-4" />
                    </Button>
                </TooltipTrigger>
                <TooltipContent>Desktop (1280px)</TooltipContent>
            </Tooltip>
            <div className="mx-1 h-4 w-px bg-border" />
            <SendTestButton sending={sending} onClick={onSendTest} />
        </div>
    )
}
