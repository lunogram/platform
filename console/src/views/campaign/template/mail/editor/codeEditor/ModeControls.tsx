import { AlertTriangle, Code2, LayoutGrid, Sparkles } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import {
    Dialog,
    DialogContent,
    DialogHeader,
    DialogFooter,
    DialogTitle,
    DialogDescription,
} from "@/components/ui/dialog"
import { isEnterprise } from "@/config/enterprise"
import type { EditorMode } from "./hooks/useEditorMode"

export function ModeToggle({
    editorMode,
    onModeSwitch,
}: {
    editorMode: EditorMode
    onModeSwitch: (mode: EditorMode) => void
}) {
    return (
        <div className="flex items-center gap-1">
            {isEnterprise && (
                <Tooltip>
                    <TooltipTrigger asChild>
                        <Button
                            variant={editorMode === "blocks" ? "secondary" : "ghost"}
                            size="sm"
                            className="h-8 w-8 p-0"
                            onClick={() => onModeSwitch("blocks")}
                        >
                            <LayoutGrid className="h-4 w-4" />
                        </Button>
                    </TooltipTrigger>
                    <TooltipContent>Block Editor</TooltipContent>
                </Tooltip>
            )}
            <Tooltip>
                <TooltipTrigger asChild>
                    <Button
                        variant={editorMode === "code" ? "secondary" : "ghost"}
                        size="sm"
                        className="h-8 w-8 p-0"
                        onClick={() => onModeSwitch("code")}
                    >
                        <Code2 className="h-4 w-4" />
                    </Button>
                </TooltipTrigger>
                <TooltipContent>Code Editor</TooltipContent>
            </Tooltip>
            {isEnterprise && (
                <Tooltip>
                    <TooltipTrigger asChild>
                        <Button
                            variant={editorMode === "builder" ? "secondary" : "ghost"}
                            size="sm"
                            className="h-8 w-8 p-0"
                            onClick={() => onModeSwitch("builder")}
                        >
                            <Sparkles className="h-4 w-4" />
                        </Button>
                    </TooltipTrigger>
                    <TooltipContent>AI Builder</TooltipContent>
                </Tooltip>
            )}
        </div>
    )
}

export function ModeSwitchDialog({
    pendingMode,
    currentMode,
    onConfirm,
    onCancel,
}: {
    pendingMode: EditorMode | null
    currentMode: EditorMode
    onConfirm: () => void
    onCancel: () => void
}) {
    return (
        <Dialog open={pendingMode !== null} onOpenChange={(open) => !open && onCancel()}>
            <DialogContent showClose={false} className="max-w-md">
                <DialogHeader>
                    <div className="mx-auto flex h-10 w-10 items-center justify-center rounded-full bg-amber-500/10">
                        <AlertTriangle className="h-5 w-5 text-amber-500" />
                    </div>
                    <DialogTitle className="text-center">Switch editor mode?</DialogTitle>
                    <DialogDescription className="text-center">
                        {currentMode === "blocks" ? (
                            <>
                                The block editor and code editor are not synchronized. Switching to
                                the code editor will remove your block layout. The generated code
                                will be preserved, but you won't be able to switch back with your
                                current layout.
                            </>
                        ) : (
                            <>
                                The block editor and code editor are not synchronized. Any code
                                changes you've made will not be reflected in the block editor. It
                                will start from its own saved state or a blank document.
                            </>
                        )}
                    </DialogDescription>
                </DialogHeader>
                <DialogFooter className="sm:flex-row sm:gap-2">
                    <Button variant="outline" className="flex-1" onClick={onCancel}>
                        Cancel
                    </Button>
                    <Button className="flex-1" onClick={onConfirm}>
                        Switch editor
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    )
}
