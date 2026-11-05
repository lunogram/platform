import { lazy, Suspense, type PropsWithChildren } from "react"
import { AIBuilderHostProvider, BuilderProvider } from "@lunogram-enterprise/ai-builder"

import { Button } from "@/components/ui/button"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import {
    Dialog,
    DialogContent,
    DialogHeader,
    DialogTitle,
    DialogDescription,
} from "@/components/ui/dialog"
import { Popover, PopoverTrigger, PopoverContent } from "@/components/ui/popover"
import {
    DropdownMenu,
    DropdownMenuTrigger,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuSeparator,
    DropdownMenuLabel,
    DropdownMenuSub,
    DropdownMenuSubTrigger,
    DropdownMenuSubContent,
} from "@/components/ui/dropdown-menu"
import { ScrollArea } from "@/components/ui/scroll-area"
import { Badge } from "@/components/ui/badge"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
    Select,
    SelectTrigger,
    SelectContent,
    SelectItem,
    SelectValue,
} from "@/components/ui/select"
import { Separator } from "@/components/ui/separator"
import { ColorPicker } from "@/components/ui/color-picker"
import { TemplateInput } from "@/components/ui/template-input"
import { MediaManager } from "@/components/media-manager"
import Modal from "@/components/modal"
import { cn, createUuid } from "@/utils"
import type { EmailDocument, BlockEditorTab } from "./hooks/useEditorMode"

// The enterprise editors render with the console's own UI components, which
// are handed to them here.

/** All host UI components passed to BlockEditorHostProvider */
const hostComponents = {
    Button,
    Dialog,
    DialogContent,
    DialogHeader,
    DialogTitle,
    DialogDescription,
    Popover,
    PopoverTrigger,
    PopoverContent,
    Tooltip,
    TooltipTrigger,
    TooltipContent,
    DropdownMenu,
    DropdownMenuTrigger,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuSeparator,
    DropdownMenuLabel,
    DropdownMenuSub,
    DropdownMenuSubTrigger,
    DropdownMenuSubContent,
    ScrollArea,
    Input,
    Label,
    Select,
    SelectTrigger,
    SelectContent,
    SelectItem,
    SelectValue,
    Separator,
    ColorPicker,
    MediaManager,
    TemplateInput,
} as const

const hostUtilities = { cn } as const

/** Host UI components for the AI builder package */
const aiBuilderHostComponents = {
    Badge,
    Button,
    Tooltip,
    TooltipTrigger,
    TooltipContent,
    ScrollArea,
    Modal,
} as const

const aiBuilderHostUtilities = { createUuid, cn } as const

/** Provides the AI builder (enterprise only) to the editor below it. */
export function BuilderHost({
    projectId,
    templateId,
    children,
}: PropsWithChildren<{ projectId: string; templateId: string }>) {
    return (
        <AIBuilderHostProvider
            components={aiBuilderHostComponents}
            utilities={aiBuilderHostUtilities}
        >
            <BuilderProvider projectId={projectId} templateId={templateId}>
                {children}
            </BuilderProvider>
        </AIBuilderHostProvider>
    )
}

interface BlockEditorProps {
    initialDocument?: EmailDocument
    onChange: (doc: EmailDocument, jsxSource: string) => void
    activeTab: BlockEditorTab
    onTabChange: (tab: BlockEditorTab) => void
    variableGroups: {
        label: string
        variables: {
            path: string
            label: string
            description?: string
            types?: string[]
            defaultValue?: unknown
        }[]
    }[]
}

/** The block editor (enterprise only), loaded the first time it is shown. */
export function BlockEditor(props: BlockEditorProps) {
    if (!LazyBlockEditor) return null
    return (
        <Suspense fallback={<div className="flex-1" />}>
            <LazyBlockEditor {...props} />
        </Suspense>
    )
}

const LazyBlockEditor = __ENTERPRISE__
    ? lazy(() =>
          import("@lunogram-enterprise/block-editor").then((mod) => ({
              default: function BlockEditorWrapped(props: BlockEditorProps) {
                  return (
                      <mod.BlockEditorHostProvider
                          components={hostComponents}
                          utilities={hostUtilities}
                          variableGroups={props.variableGroups}
                      >
                          <mod.BlockEditor
                              initialDocument={props.initialDocument}
                              onChange={props.onChange}
                              activeTab={props.activeTab}
                              onTabChange={props.onTabChange}
                          />
                      </mod.BlockEditorHostProvider>
                  )
              },
          })),
      )
    : null
