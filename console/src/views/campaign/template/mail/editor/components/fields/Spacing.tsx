/* eslint-disable react-hooks/rules-of-hooks */
import { createUsePuck, type Field } from "@measured/puck";
import { getViewportTailwindBreakpoint } from "../../viewport";
import { addUnit, hasAnyProperty } from "./unit";
import { useTranslation } from "react-i18next";
import { Link2, Link2Off, Plus, Minus } from "lucide-react";

import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
export interface LayoutViewport {
    paddingTop?: string;
    paddingBottom?: string;
    paddingLeft?: string;
    paddingRight?: string;
    marginTop?: string;
    marginBottom?: string;
    marginLeft?: string;
    marginRight?: string;
    paddingLinked?: boolean;
    marginLinked?: boolean;
}

export interface SpacingProps {
    sm?: Partial<LayoutViewport>;
    md?: Partial<LayoutViewport>;
    xl?: Partial<LayoutViewport>;
}

export const spacingClassMap: Record<Exclude<keyof LayoutViewport, 'paddingLinked' | 'marginLinked'>, (value: string, prefix: string) => string> = {
    paddingTop: (value, prefix) => `${prefix}pt-${addUnit(value)}`,
    paddingRight: (value, prefix) => `${prefix}pr-${addUnit(value)}`,
    paddingBottom: (value, prefix) => `${prefix}pb-${addUnit(value)}`,
    paddingLeft: (value, prefix) => `${prefix}pl-${addUnit(value)}`,
    marginTop: (value, prefix) => `${prefix}mt-${addUnit(value)}`,
    marginRight: (value, prefix) => `${prefix}mr-${addUnit(value)}`,
    marginBottom: (value, prefix) => `${prefix}mb-${addUnit(value)}`,
    marginLeft: (value, prefix) => `${prefix}ml-${addUnit(value)}`,
};

const usePuck = createUsePuck();

export const Spacing: Field<SpacingProps, SpacingProps> = {
    type: "custom",
    render: ({ onChange, value = {} }) => {
        const { t } = useTranslation();
        const viewport = usePuck((s) => s.appState.ui.viewports.current);
        const breakpoint = getViewportTailwindBreakpoint(viewport.width);

        const config = value[breakpoint] || {};
        const paddingLinked = config.paddingLinked ?? true;
        const marginLinked = config.marginLinked ?? true;

        // Check if padding or margin values exist (including empty strings which means fields are enabled)
        const hasPadding = hasAnyProperty(config, ['paddingTop', 'paddingRight', 'paddingBottom', 'paddingLeft']);
        const hasMargin = hasAnyProperty(config, ['marginTop', 'marginRight', 'marginBottom', 'marginLeft']);

        const handlePaddingChange = (field: string, val: string) => {
            onChange({
                ...value,
                [breakpoint]: {
                    ...config,
                    ...(paddingLinked ? {
                        paddingTop: val,
                        paddingRight: val,
                        paddingBottom: val,
                        paddingLeft: val
                    } : {
                        [field]: val
                    })
                }
            });
        };

        const handleMarginChange = (field: string, val: string) => {
            onChange({
                ...value,
                [breakpoint]: {
                    ...config,
                    ...(marginLinked ? {
                        marginTop: val,
                        marginRight: val,
                        marginBottom: val,
                        marginLeft: val
                    } : {
                        [field]: val
                    })
                }
            });
        };

        const handleAddPadding = () => {
            onChange({
                ...value,
                [breakpoint]: {
                    ...config,
                    paddingTop: '0',
                    paddingRight: '0',
                    paddingBottom: '0',
                    paddingLeft: '0'
                }
            });
        };

        const handleRemovePadding = () => {
            // eslint-disable-next-line @typescript-eslint/no-unused-vars
            const { paddingTop, paddingRight, paddingBottom, paddingLeft, paddingLinked, ...rest } = config;
            onChange({
                ...value,
                [breakpoint]: rest
            });
        };

        const handleAddMargin = () => {
            onChange({
                ...value,
                [breakpoint]: {
                    ...config,
                    marginTop: '0',
                    marginRight: '0',
                    marginBottom: '0',
                    marginLeft: '0'
                }
            });
        };

        const handleRemoveMargin = () => {
            // eslint-disable-next-line @typescript-eslint/no-unused-vars
            const { marginTop, marginRight, marginBottom, marginLeft, marginLinked, ...rest } = config;
            onChange({
                ...value,
                [breakpoint]: rest
            });
        };

        const paddingFields: Array<{ key: Exclude<keyof LayoutViewport, 'paddingLinked' | 'marginLinked'>; label: string; placeholder: string }> = [
            { key: 'paddingTop', label: t('editor.fields.spacing.top'), placeholder: t('editor.fields.spacing.placeholder') },
            { key: 'paddingRight', label: t('editor.fields.spacing.right'), placeholder: t('editor.fields.spacing.placeholder') },
            { key: 'paddingBottom', label: t('editor.fields.spacing.bottom'), placeholder: t('editor.fields.spacing.placeholder') },
            { key: 'paddingLeft', label: t('editor.fields.spacing.left'), placeholder: t('editor.fields.spacing.placeholder') }
        ];

        const marginFields: Array<{ key: Exclude<keyof LayoutViewport, 'paddingLinked' | 'marginLinked'>; label: string; placeholder: string }> = [
            { key: 'marginTop', label: t('editor.fields.spacing.top'), placeholder: t('editor.fields.spacing.placeholder') },
            { key: 'marginRight', label: t('editor.fields.spacing.right'), placeholder: t('editor.fields.spacing.placeholder') },
            { key: 'marginBottom', label: t('editor.fields.spacing.bottom'), placeholder: t('editor.fields.spacing.placeholder') },
            { key: 'marginLeft', label: t('editor.fields.spacing.left'), placeholder: t('editor.fields.spacing.placeholder') }
        ];

        return (
            <div className="space-y-4">
                {/* Padding Section */}
                {hasPadding ? (
                    <div>
                        <div className="flex items-center justify-between mb-2">
                            <h4 className="text-xs font-semibold text-gray-700 uppercase tracking-wide">{t('editor.fields.spacing.padding')}</h4>
                            <Button
                                type="button"
                                variant="ghost"
                                size="icon"
                                className="h-6 w-6"
                                onClick={handleRemovePadding}
                            >
                                <Minus className="h-3 w-3" />
                            </Button>
                        </div>
                        <div className="grid grid-cols-[1fr_auto] gap-2 items-start">
                            <div className="grid grid-cols-4 gap-2">
                                {paddingFields.map(field => (
                                    <div key={field.key} className="space-y-1">
                                        <label className="text-xs font-medium text-gray-600">
                                            {field.label}
                                        </label>
                                        <Input
                                            value={config[field.key] ?? ''}
                                            onChange={(e) => handlePaddingChange(field.key, e.target.value)}
                                            placeholder={field.placeholder}
                                            className="h-8 text-sm"
                                        />
                                    </div>
                                ))}
                            </div>
                            <div className="flex items-end h-full pt-[20px]">
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
                                                paddingLinked: !paddingLinked
                                            }
                                        });
                                    }}
                                >
                                    {paddingLinked ? <Link2 className="h-4 w-4" /> : <Link2Off className="h-4 w-4" />}
                                </Button>
                            </div>
                        </div>
                    </div>
                ) : (
                    <div className="flex items-center justify-between">
                        <h4 className="text-xs font-semibold text-gray-700 uppercase tracking-wide">{t('editor.fields.spacing.padding')}</h4>
                        <Button
                            type="button"
                            variant="ghost"
                            size="icon"
                            className="h-6 w-6"
                            onClick={handleAddPadding}
                        >
                            <Plus className="h-3 w-3" />
                        </Button>
                    </div>
                )}

                {/* Margin Section */}
                {hasMargin ? (
                    <div>
                        <div className="flex items-center justify-between mb-2">
                            <h4 className="text-xs font-semibold text-gray-700 uppercase tracking-wide">{t('editor.fields.spacing.margin')}</h4>
                            <Button
                                type="button"
                                variant="ghost"
                                size="icon"
                                className="h-6 w-6"
                                onClick={handleRemoveMargin}
                            >
                                <Minus className="h-3 w-3" />
                            </Button>
                        </div>
                        <div className="grid grid-cols-[1fr_auto] gap-2 items-start">
                            <div className="grid grid-cols-4 gap-2">
                                {marginFields.map(field => (
                                    <div key={field.key} className="space-y-1">
                                        <label className="text-xs font-medium text-gray-600">
                                            {field.label}
                                        </label>
                                        <Input
                                            value={config[field.key] ?? ''}
                                            onChange={(e) => handleMarginChange(field.key, e.target.value)}
                                            placeholder={field.placeholder}
                                            className="h-8 text-sm"
                                        />
                                    </div>
                                ))}
                            </div>
                            <div className="flex items-end h-full pt-[20px]">
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
                                                marginLinked: !marginLinked
                                            }
                                        });
                                    }}
                                >
                                    {marginLinked ? <Link2 className="h-4 w-4" /> : <Link2Off className="h-4 w-4" />}
                                </Button>
                            </div>
                        </div>
                    </div>
                ) : (
                    <div className="flex items-center justify-between">
                        <h4 className="text-xs font-semibold text-gray-700 uppercase tracking-wide">{t('editor.fields.spacing.margin')}</h4>
                        <Button
                            type="button"
                            variant="ghost"
                            size="icon"
                            className="h-6 w-6"
                            onClick={handleAddMargin}
                        >
                            <Plus className="h-3 w-3" />
                        </Button>
                    </div>
                )}
            </div>
        );
    }
}
