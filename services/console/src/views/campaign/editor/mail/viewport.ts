export const viewports = [
    { width: 360, icon: "Smartphone", label: "Small", tailwind: "sm" },
    { width: 768, icon: "Tablet", label: "Medium", tailwind: "md" },
    { width: 1280, icon: "Monitor", label: "Large", tailwind: "xl" },
];

export function getViewportTailwindBreakpoint(width: number) {
    if (width <= 360) return "sm";
    if (width <= 768) return "md";
    return "xl";
}
