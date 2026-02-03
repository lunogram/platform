/* eslint-disable react-hooks/rules-of-hooks */
import { createUsePuck, type Field } from "@measured/puck";
import { getViewportTailwindBreakpoint } from "../../viewport";
import { addUnit, hasAnyProperty } from "./unit";
import { useTranslation } from "react-i18next";
import { Link2, Link2Off, Plus, Minus } from "lucide-react";

import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Button } from "@/components/ui/button";

export interface DecorationViewport {
    backgroundColor?: string;
    borderTopLeftRadius?: string;
    borderTopRightRadius?: string;
    borderBottomLeftRadius?: string;
    borderBottomRightRadius?: string;
    borderTopWidth?: string;
    borderRightWidth?: string;
    borderBottomWidth?: string;
    borderLeftWidth?: string;
    borderColor?: string;
    borderStyle?: string;
    borderRadiusLinked?: boolean;
    borderWidthLinked?: boolean;
}

export interface DecorationProps {
    sm?: Partial<DecorationViewport>;
    md?: Partial<DecorationViewport>;
    xl?: Partial<DecorationViewport>;
}

export const decorationClassMap: Record<Exclude<keyof DecorationViewport, 'borderRadiusLinked' | 'borderWidthLinked'>, (value: string, prefix: string) => string> = {
    backgroundColor: (value, prefix) => `${prefix}bg-[${value}]`,
    borderTopLeftRadius: (value, prefix) => `${prefix}rounded-tl-${addUnit(value)}`,
    borderTopRightRadius: (value, prefix) => `${prefix}rounded-tr-${addUnit(value)}`,
    borderBottomLeftRadius: (value, prefix) => `${prefix}rounded-bl-${addUnit(value)}`,
    borderBottomRightRadius: (value, prefix) => `${prefix}rounded-br-${addUnit(value)}`,
    borderStyle: (value, prefix) => `${prefix}border-${value}`,
    borderColor: (value, prefix) => `${prefix}border-[${value}]`,
    borderTopWidth: (value, prefix) => `${prefix}border-t-${addUnit(value)}`,
    borderRightWidth: (value, prefix) => `${prefix}border-r-${addUnit(value)}`,
    borderBottomWidth: (value, prefix) => `${prefix}border-b-${addUnit(value)}`,
    borderLeftWidth: (value, prefix) => `${prefix}border-l-${addUnit(value)}`,
};

const usePuck = createUsePuck();

export const Decoration: Field<DecorationProps, DecorationProps> = {
    type: "custom",
    render: ({ onChange, value = {} }) => {
        const { t } = useTranslation();
        const viewport = usePuck((s) => s.appState.ui.viewports.current);
        const breakpoint = getViewportTailwindBreakpoint(viewport.width);

        const config = value[breakpoint] || {};
        const borderRadiusLinked = config.borderRadiusLinked ?? true;
        const borderWidthLinked = config.borderWidthLinked ?? true;

        // Check if sections have values
        const hasBackground = hasAnyProperty(config, ['backgroundColor']);
        const hasBorderRadius = hasAnyProperty(config, ['borderTopLeftRadius', 'borderTopRightRadius', 'borderBottomLeftRadius', 'borderBottomRightRadius']);
        const hasBorder = hasAnyProperty(config, ['borderTopWidth', 'borderRightWidth', 'borderBottomWidth', 'borderLeftWidth', 'borderStyle', 'borderColor']);

        const handleChange = (field: string, val: string) => {
            onChange({
                ...value,
                [breakpoint]: {
                    ...config,
                    [field]: val
                }
            });
        };

        const handleBorderRadiusChange = (field: string, val: string) => {
            onChange({
                ...value,
                [breakpoint]: {
                    ...config,
                    ...(borderRadiusLinked ? {
                        borderTopLeftRadius: val,
                        borderTopRightRadius: val,
                        borderBottomLeftRadius: val,
                        borderBottomRightRadius: val
                    } : {
                        [field]: val
                    })
                }
            });
        };

        const handleBorderWidthChange = (field: string, val: string) => {
            onChange({
                ...value,
                [breakpoint]: {
                    ...config,
                    ...(borderWidthLinked ? {
                        borderTopWidth: val,
                        borderRightWidth: val,
                        borderBottomWidth: val,
                        borderLeftWidth: val
                    } : {
                        [field]: val
                    })
                }
            });
        };

        const handleAddBackground = () => {
            onChange({
                ...value,
                [breakpoint]: {
                    ...config,
                    backgroundColor: '#ffffff'
                }
            });
        };

        const handleRemoveBackground = () => {
            // eslint-disable-next-line @typescript-eslint/no-unused-vars
            const { backgroundColor, ...rest } = config;
            onChange({
                ...value,
                [breakpoint]: rest
            });
        };

        const handleAddBorderRadius = () => {
            onChange({
                ...value,
                [breakpoint]: {
                    ...config,
                    borderTopLeftRadius: '0',
                    borderTopRightRadius: '0',
                    borderBottomLeftRadius: '0',
                    borderBottomRightRadius: '0'
                }
            });
        };

        const handleRemoveBorderRadius = () => {
            // eslint-disable-next-line @typescript-eslint/no-unused-vars
            const { borderTopLeftRadius, borderTopRightRadius, borderBottomLeftRadius, borderBottomRightRadius, borderRadiusLinked, ...rest } = config;
            onChange({
                ...value,
                [breakpoint]: rest
            });
        };

        const handleAddBorder = () => {
            onChange({
                ...value,
                [breakpoint]: {
                    ...config,
                    borderTopWidth: '0',
                    borderRightWidth: '0',
                    borderBottomWidth: '0',
                    borderLeftWidth: '0',
                    borderStyle: 'solid',
                    borderColor: '#000000'
                }
            });
        };

        const handleRemoveBorder = () => {
            // eslint-disable-next-line @typescript-eslint/no-unused-vars
            const { borderTopWidth, borderRightWidth, borderBottomWidth, borderLeftWidth, borderStyle, borderColor, borderWidthLinked, ...rest } = config;
            onChange({
                ...value,
                [breakpoint]: rest
            });
        };

        const borderStyles = [
            { value: 'solid', label: t('editor.fields.decoration.border_styles.solid') },
            { value: 'dashed', label: t('editor.fields.decoration.border_styles.dashed') },
            { value: 'dotted', label: t('editor.fields.decoration.border_styles.dotted') },
            { value: 'double', label: t('editor.fields.decoration.border_styles.double') },
            { value: 'none', label: t('editor.fields.decoration.border_styles.none') },
        ];

        return (
            <div className="space-y-4">
                {/* Background Section */}
                {hasBackground ? (
                    <div>
                        <div className="flex items-center justify-between mb-2">
                            <h4 className="text-xs font-semibold text-gray-700 uppercase tracking-wide">{t('editor.fields.decoration.background')}</h4>
                            <Button
                                type="button"
                                variant="ghost"
                                size="icon"
                                className="h-6 w-6"
                                onClick={handleRemoveBackground}
                            >
                                <Minus className="h-3 w-3" />
                            </Button>
                        </div>
                        <div className="space-y-1">
                            <label className="text-xs font-medium text-gray-600">{t('editor.fields.decoration.background_color')}</label>
                            <Input
                                type="color"
                                value={config.backgroundColor ?? '#ffffff'}
                                onChange={(e) => handleChange('backgroundColor', e.target.value)}
                                className="h-8"
                            />
                        </div>
                    </div>
                ) : (
                    <div className="flex items-center justify-between">
                        <h4 className="text-xs font-semibold text-gray-700 uppercase tracking-wide">{t('editor.fields.decoration.background')}</h4>
                        <Button
                            type="button"
                            variant="ghost"
                            size="icon"
                            className="h-6 w-6"
                            onClick={handleAddBackground}
                        >
                            <Plus className="h-3 w-3" />
                        </Button>
                    </div>
                )}

                {/* Border Radius Section */}
                {hasBorderRadius ? (
                    <div>
                        <div className="flex items-center justify-between mb-2">
                            <h4 className="text-xs font-semibold text-gray-700 uppercase tracking-wide">{t('editor.fields.decoration.border_radius')}</h4>
                            <Button
                                type="button"
                                variant="ghost"
                                size="icon"
                                className="h-6 w-6"
                                onClick={handleRemoveBorderRadius}
                            >
                                <Minus className="h-3 w-3" />
                            </Button>
                        </div>
                        <div className="grid grid-cols-[1fr_auto] gap-2 items-center">
                            <div className="grid grid-cols-2 gap-2">
                                <div className="relative">
                                    <div className="absolute left-2 top-1/2 -translate-y-1/2 pointer-events-none">
                                        <div className="w-3 h-3 border-2 border-gray-400 rounded-tl-md" />
                                    </div>
                                    <Input
                                        value={config.borderTopLeftRadius ?? ''}
                                        onChange={(e) => handleBorderRadiusChange('borderTopLeftRadius', e.target.value)}
                                        placeholder="0"
                                        className="h-8 text-sm pl-7"
                                        title={t('editor.fields.decoration.top_left')}
                                    />
                                </div>
                                <div className="relative">
                                    <div className="absolute left-2 top-1/2 -translate-y-1/2 pointer-events-none">
                                        <div className="w-3 h-3 border-2 border-gray-400 rounded-tr-md" />
                                    </div>
                                    <Input
                                        value={config.borderTopRightRadius ?? ''}
                                        onChange={(e) => handleBorderRadiusChange('borderTopRightRadius', e.target.value)}
                                        placeholder="0"
                                        className="h-8 text-sm pl-7"
                                        title={t('editor.fields.decoration.top_right')}
                                    />
                                </div>
                                <div className="relative">
                                    <div className="absolute left-2 top-1/2 -translate-y-1/2 pointer-events-none">
                                        <div className="w-3 h-3 border-2 border-gray-400 rounded-bl-md" />
                                    </div>
                                    <Input
                                        value={config.borderBottomLeftRadius ?? ''}
                                        onChange={(e) => handleBorderRadiusChange('borderBottomLeftRadius', e.target.value)}
                                        placeholder="0"
                                        className="h-8 text-sm pl-7"
                                        title={t('editor.fields.decoration.bottom_left')}
                                    />
                                </div>
                                <div className="relative">
                                    <div className="absolute left-2 top-1/2 -translate-y-1/2 pointer-events-none">
                                        <div className="w-3 h-3 border-2 border-gray-400 rounded-br-md" />
                                    </div>
                                    <Input
                                        value={config.borderBottomRightRadius ?? ''}
                                        onChange={(e) => handleBorderRadiusChange('borderBottomRightRadius', e.target.value)}
                                        placeholder="0"
                                        className="h-8 text-sm pl-7"
                                        title={t('editor.fields.decoration.bottom_right')}
                                    />
                                </div>
                            </div>
                            <Button
                                type="button"
                                variant="ghost"
                                size="icon"
                                className="h-8 w-8"
                                onClick={() => {
                                    onChange({
                                        ...value,
                                        [breakpoint]: {
                                            ...config,
                                            borderRadiusLinked: !borderRadiusLinked
                                        }
                                    });
                                }}
                            >
                                {borderRadiusLinked ? <Link2 className="h-4 w-4" /> : <Link2Off className="h-4 w-4" />}
                            </Button>
                        </div>
                    </div>
                ) : (
                    <div className="flex items-center justify-between">
                        <h4 className="text-xs font-semibold text-gray-700 uppercase tracking-wide">{t('editor.fields.decoration.border_radius')}</h4>
                        <Button
                            type="button"
                            variant="ghost"
                            size="icon"
                            className="h-6 w-6"
                            onClick={handleAddBorderRadius}
                        >
                            <Plus className="h-3 w-3" />
                        </Button>
                    </div>
                )}

                {/* Border Section */}
                {hasBorder ? (
                    <div>
                        <div className="flex items-center justify-between mb-2">
                            <h4 className="text-xs font-semibold text-gray-700 uppercase tracking-wide">{t('editor.fields.decoration.border')}</h4>
                            <Button
                                type="button"
                                variant="ghost"
                                size="icon"
                                className="h-6 w-6"
                                onClick={handleRemoveBorder}
                            >
                                <Minus className="h-3 w-3" />
                            </Button>
                        </div>
                        <div className="grid grid-cols-2 gap-2 mb-2">
                            <div className="space-y-1">
                                <label className="text-xs font-medium text-gray-600">{t('editor.fields.decoration.border_style')}</label>
                                <Select value={config.borderStyle ?? ''} onValueChange={(val) => handleChange('borderStyle', val)}>
                                    <SelectTrigger className="h-8 text-sm">
                                        <SelectValue placeholder={t('editor.fields.decoration.border_style_placeholder')} />
                                    </SelectTrigger>
                                    <SelectContent>
                                        {borderStyles.map(style => (
                                            <SelectItem key={style.value} value={style.value}>{style.label}</SelectItem>
                                        ))}
                                    </SelectContent>
                                </Select>
                            </div>
                            <div className="space-y-1">
                                <label className="text-xs font-medium text-gray-600">{t('editor.fields.decoration.border_color')}</label>
                                <Input
                                    type="color"
                                    value={config.borderColor ?? '#000000'}
                                    onChange={(e) => handleChange('borderColor', e.target.value)}
                                    className="h-8"
                                />
                            </div>
                        </div>
                        <div className="grid grid-cols-[1fr_auto] gap-2 items-center">
                            <div className="grid grid-cols-2 gap-2">
                                <div className="relative">
                                    <div className="absolute left-2 top-1/2 -translate-y-1/2 pointer-events-none">
                                        <div className="w-3 h-0.5 bg-gray-400" />
                                    </div>
                                    <Input
                                        value={config.borderTopWidth ?? ''}
                                        onChange={(e) => handleBorderWidthChange('borderTopWidth', e.target.value)}
                                        placeholder="0"
                                        className="h-8 text-sm pl-7"
                                        title={t('editor.fields.spacing.top')}
                                    />
                                </div>
                                <div className="relative">
                                    <div className="absolute left-2 top-1/2 -translate-y-1/2 pointer-events-none">
                                        <div className="w-0.5 h-3 bg-gray-400" />
                                    </div>
                                    <Input
                                        value={config.borderRightWidth ?? ''}
                                        onChange={(e) => handleBorderWidthChange('borderRightWidth', e.target.value)}
                                        placeholder="0"
                                        className="h-8 text-sm pl-7"
                                        title={t('editor.fields.spacing.right')}
                                    />
                                </div>
                                <div className="relative">
                                    <div className="absolute left-2 top-1/2 -translate-y-1/2 pointer-events-none">
                                        <div className="w-3 h-0.5 bg-gray-400" />
                                    </div>
                                    <Input
                                        value={config.borderBottomWidth ?? ''}
                                        onChange={(e) => handleBorderWidthChange('borderBottomWidth', e.target.value)}
                                        placeholder="0"
                                        className="h-8 text-sm pl-7"
                                        title={t('editor.fields.spacing.bottom')}
                                    />
                                </div>
                                <div className="relative">
                                    <div className="absolute left-2 top-1/2 -translate-y-1/2 pointer-events-none">
                                        <div className="w-0.5 h-3 bg-gray-400" />
                                    </div>
                                    <Input
                                        value={config.borderLeftWidth ?? ''}
                                        onChange={(e) => handleBorderWidthChange('borderLeftWidth', e.target.value)}
                                        placeholder="0"
                                        className="h-8 text-sm pl-7"
                                        title={t('editor.fields.spacing.left')}
                                    />
                                </div>
                            </div>
                            <Button
                                type="button"
                                variant="ghost"
                                size="icon"
                                className="h-8 w-8"
                                onClick={() => {
                                    onChange({
                                        ...value,
                                        [breakpoint]: {
                                            ...config,
                                            borderWidthLinked: !borderWidthLinked
                                        }
                                    });
                                }}
                            >
                                {borderWidthLinked ? <Link2 className="h-4 w-4" /> : <Link2Off className="h-4 w-4" />}
                            </Button>
                        </div>
                    </div>
                ) : (
                    <div className="flex items-center justify-between">
                        <h4 className="text-xs font-semibold text-gray-700 uppercase tracking-wide">{t('editor.fields.decoration.border')}</h4>
                        <Button
                            type="button"
                            variant="ghost"
                            size="icon"
                            className="h-6 w-6"
                            onClick={handleAddBorder}
                        >
                            <Plus className="h-3 w-3" />
                        </Button>
                    </div>
                )}
            </div>
        );
    }
}
