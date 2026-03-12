export type Viewport = "mobile" | "tablet" | "desktop"
export type EditorTab = "code" | "plaintext"

export const VIEWPORT_WIDTHS: Record<Viewport, number> = {
    mobile: 375,
    tablet: 768,
    desktop: 1280,
}
