/* eslint-disable react-hooks/rules-of-hooks */
import { createUsePuck, type Field } from "@measured/puck";
import { getViewportTailwindBreakpoint } from "../../viewport";
import { addUnit, hasAnyProperty } from "./unit";
import { useTranslation } from "react-i18next";
import { Plus, Minus } from "lucide-react";

import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Button } from "@/components/ui/button";

export interface TypographyViewport {
    fontFamily?: string;
    fontSize?: string;
    fontWeight?: string;
    fontStyle?: string;
    lineHeight?: string;
    letterSpacing?: string;
    textAlign?: string;
    textDecoration?: string;
    textTransform?: string;
    color?: string;
}

export interface TypographyProps {
    sm?: Partial<TypographyViewport>;
    md?: Partial<TypographyViewport>;
    xl?: Partial<TypographyViewport>;
}

export const typographyClassMap: Record<keyof TypographyViewport, (value: string, prefix: string) => string> = {
    fontFamily: (value, prefix) => `${prefix}font-${value}`,
    fontSize: (value, prefix) => `${prefix}text-${addUnit(value)}`,
    fontWeight: (value, prefix) => `${prefix}font-[${value}]`,
    fontStyle: (value, prefix) => `${prefix}${value}`,
    lineHeight: (value, prefix) => `${prefix}leading-${addUnit(value)}`,
    letterSpacing: (value, prefix) => `${prefix}tracking-${addUnit(value)}`,
    textAlign: (value, prefix) => `${prefix}text-${value}`,
    textDecoration: (value, prefix) => `${prefix}${value}`,
    textTransform: (value, prefix) => `${prefix}${value}`,
    color: (value, prefix) => `${prefix}text-[${value}]`,
};

const usePuck = createUsePuck();

export const Typography: Field<TypographyProps, TypographyProps> = {
    type: "custom",
    render: ({ onChange, value = {} }) => {
        const { t } = useTranslation();
        const viewport = usePuck((s) => s.appState.ui.viewports.current);
        const breakpoint = getViewportTailwindBreakpoint(viewport.width);

        const config = value[breakpoint] || {};

        // Check if any typography values exist
        const hasTypography = hasAnyProperty(config, [
            'fontFamily', 'fontSize', 'fontWeight', 'fontStyle', 'lineHeight',
            'letterSpacing', 'textAlign', 'textDecoration', 'textTransform', 'color'
        ]);

        const handleChange = (field: string, val: string) => {
            onChange({
                ...value,
                [breakpoint]: {
                    ...config,
                    [field]: val
                }
            });
        };


        const handleAddTypography = () => {
            onChange({
                ...value,
                [breakpoint]: {
                    ...config,
                    fontSize: '16',
                    color: '#000000'
                }
            });
        };

        const handleRemoveTypography = () => {
            // eslint-disable-next-line @typescript-eslint/no-unused-vars
            const { fontFamily, fontSize, fontWeight, fontStyle, lineHeight, letterSpacing, textAlign, textDecoration, textTransform, color, ...rest } = config;
            onChange({
                ...value,
                [breakpoint]: rest
            });
        };

        const fontFamilies = [
            { value: 'sans', label: t('editor.fields.typography.font_families.sans') },
            { value: 'serif', label: t('editor.fields.typography.font_families.serif') },
            { value: 'mono', label: t('editor.fields.typography.font_families.mono') },
        ];

        const fontWeights = [
            { value: 'thin', label: t('editor.fields.typography.font_weights.100') },
            { value: 'extralight', label: t('editor.fields.typography.font_weights.200') },
            { value: 'light', label: t('editor.fields.typography.font_weights.300') },
            { value: 'normal', label: t('editor.fields.typography.font_weights.400') },
            { value: 'medium', label: t('editor.fields.typography.font_weights.500') },
            { value: 'semibold', label: t('editor.fields.typography.font_weights.600') },
            { value: 'bold', label: t('editor.fields.typography.font_weights.700') },
            { value: 'extrabold', label: t('editor.fields.typography.font_weights.800') },
            { value: 'black', label: t('editor.fields.typography.font_weights.900') },
        ];

        const fontStyles = [
            { value: 'normal', label: t('editor.fields.typography.font_styles.normal') },
            { value: 'italic', label: t('editor.fields.typography.font_styles.italic') },
        ];

        const textAligns = [
            { value: 'left', label: t('editor.fields.typography.text_aligns.left') },
            { value: 'center', label: t('editor.fields.typography.text_aligns.center') },
            { value: 'right', label: t('editor.fields.typography.text_aligns.right') },
            { value: 'justify', label: t('editor.fields.typography.text_aligns.justify') },
        ];

        const textDecorations = [
            { value: 'none', label: t('editor.fields.typography.text_decorations.none') },
            { value: 'underline', label: t('editor.fields.typography.text_decorations.underline') },
            { value: 'line-through', label: t('editor.fields.typography.text_decorations.line_through') },
        ];

        const textTransforms = [
            { value: 'none', label: t('editor.fields.typography.text_transforms.none') },
            { value: 'uppercase', label: t('editor.fields.typography.text_transforms.uppercase') },
            { value: 'lowercase', label: t('editor.fields.typography.text_transforms.lowercase') },
            { value: 'capitalize', label: t('editor.fields.typography.text_transforms.capitalize') },
        ];

        return (
            <div className="space-y-4">
                {hasTypography ? (
                    <div>
                        <div className="flex items-center justify-between mb-2">
                            <h4 className="text-xs font-semibold text-gray-700 uppercase tracking-wide">{t('editor.fields.typography.title')}</h4>
                            <Button
                                type="button"
                                variant="ghost"
                                size="icon"
                                className="h-6 w-6"
                                onClick={handleRemoveTypography}
                            >
                                <Minus className="h-3 w-3" />
                            </Button>
                        </div>
                        <div className="space-y-4">
                            <div className="grid grid-cols-2 gap-2">
                                <div className="space-y-1">
                                    <label className="text-xs font-medium text-gray-600">{t('editor.fields.typography.font_family')}</label>
                                    <Select value={config.fontFamily ?? ''} onValueChange={(val) => handleChange('fontFamily', val)}>
                                        <SelectTrigger className="h-8 text-sm">
                                            <SelectValue placeholder={t('editor.fields.typography.font_family_placeholder')} />
                                        </SelectTrigger>
                                        <SelectContent>
                                            {fontFamilies.map(font => (
                                                <SelectItem key={font.value} value={font.value}>{font.label}</SelectItem>
                                            ))}
                                        </SelectContent>
                                    </Select>
                                </div>

                                <div className="space-y-1">
                                    <label className="text-xs font-medium text-gray-600">{t('editor.fields.typography.font_weight')}</label>
                                    <Select value={config.fontWeight ?? ''} onValueChange={(val) => handleChange('fontWeight', val)}>
                                        <SelectTrigger className="h-8 text-sm">
                                            <SelectValue placeholder={t('editor.fields.typography.font_weight_placeholder')} />
                                        </SelectTrigger>
                                        <SelectContent>
                                            {fontWeights.map(weight => (
                                                <SelectItem key={weight.value} value={weight.value}>{weight.label}</SelectItem>
                                            ))}
                                        </SelectContent>
                                    </Select>
                                </div>
                            </div>

                            <div className="grid grid-cols-2 gap-2">
                                <div className="space-y-1">
                                    <label className="text-xs font-medium text-gray-600">{t('editor.fields.typography.font_style')}</label>
                                    <Select value={config.fontStyle ?? ''} onValueChange={(val) => handleChange('fontStyle', val)}>
                                        <SelectTrigger className="h-8 text-sm">
                                            <SelectValue placeholder={t('editor.fields.typography.font_style_placeholder')} />
                                        </SelectTrigger>
                                        <SelectContent>
                                            {fontStyles.map(style => (
                                                <SelectItem key={style.value} value={style.value}>{style.label}</SelectItem>
                                            ))}
                                        </SelectContent>
                                    </Select>
                                </div>

                                <div className="space-y-1">
                                    <label className="text-xs font-medium text-gray-600">{t('editor.fields.typography.font_size')}</label>
                                    <Input
                                        value={config.fontSize ?? ''}
                                        onChange={(e) => handleChange('fontSize', e.target.value)}
                                        placeholder={t('editor.fields.typography.font_size_placeholder')}
                                        className="h-8 text-sm"
                                    />
                                </div>
                            </div>

                            <div className="grid grid-cols-2 gap-2">
                                <div className="space-y-1">
                                    <label className="text-xs font-medium text-gray-600">{t('editor.fields.typography.color')}</label>
                                    <Input
                                        type="color"
                                        value={config.color ?? '#000000'}
                                        onChange={(e) => handleChange('color', e.target.value)}
                                        className="h-8"
                                    />
                                </div>
                            </div>

                            <div className="grid grid-cols-2 gap-2">
                                <div className="space-y-1">
                                    <label className="text-xs font-medium text-gray-600">{t('editor.fields.typography.letter_spacing')}</label>
                                    <Input
                                        value={config.letterSpacing ?? ''}
                                        onChange={(e) => handleChange('letterSpacing', e.target.value)}
                                        placeholder={t('editor.fields.typography.letter_spacing_placeholder')}
                                        className="h-8 text-sm"
                                    />
                                </div>

                                <div className="space-y-1">
                                    <label className="text-xs font-medium text-gray-600">{t('editor.fields.typography.line_height')}</label>
                                    <Input
                                        value={config.lineHeight ?? ''}
                                        onChange={(e) => handleChange('lineHeight', e.target.value)}
                                        placeholder={t('editor.fields.typography.line_height_placeholder')}
                                        className="h-8 text-sm"
                                    />
                                </div>
                            </div>

                            <div className="grid grid-cols-2 gap-2">
                                <div className="space-y-1">
                                    <label className="text-xs font-medium text-gray-600">{t('editor.fields.typography.text_align')}</label>
                                    <Select value={config.textAlign ?? ''} onValueChange={(val) => handleChange('textAlign', val)}>
                                        <SelectTrigger className="h-8 text-sm">
                                            <SelectValue placeholder={t('editor.fields.typography.text_align_placeholder')} />
                                        </SelectTrigger>
                                        <SelectContent>
                                            {textAligns.map(align => (
                                                <SelectItem key={align.value} value={align.value}>{align.label}</SelectItem>
                                            ))}
                                        </SelectContent>
                                    </Select>
                                </div>

                                <div className="space-y-1">
                                    <label className="text-xs font-medium text-gray-600">{t('editor.fields.typography.text_decoration')}</label>
                                    <Select value={config.textDecoration ?? ''} onValueChange={(val) => handleChange('textDecoration', val)}>
                                        <SelectTrigger className="h-8 text-sm">
                                            <SelectValue placeholder={t('editor.fields.typography.text_decoration_placeholder')} />
                                        </SelectTrigger>
                                        <SelectContent>
                                            {textDecorations.map(decoration => (
                                                <SelectItem key={decoration.value} value={decoration.value}>{decoration.label}</SelectItem>
                                            ))}
                                        </SelectContent>
                                    </Select>
                                </div>
                            </div>

                            <div className="space-y-1">
                                <label className="text-xs font-medium text-gray-600">{t('editor.fields.typography.text_transform')}</label>
                                <Select value={config.textTransform ?? ''} onValueChange={(val) => handleChange('textTransform', val)}>
                                    <SelectTrigger className="h-8 text-sm">
                                        <SelectValue placeholder={t('editor.fields.typography.text_transform_placeholder')} />
                                    </SelectTrigger>
                                    <SelectContent>
                                        {textTransforms.map(transform => (
                                            <SelectItem key={transform.value} value={transform.value}>{transform.label}</SelectItem>
                                        ))}
                                    </SelectContent>
                                </Select>
                            </div>
                        </div>
                    </div>
                ) : (
                    <div className="flex items-center justify-between">
                        <h4 className="text-xs font-semibold text-gray-700 uppercase tracking-wide">{t('editor.fields.typography.title')}</h4>
                        <Button
                            type="button"
                            variant="ghost"
                            size="icon"
                            className="h-6 w-6"
                            onClick={handleAddTypography}
                        >
                            <Plus className="h-3 w-3" />
                        </Button>
                    </div>
                )}
            </div>
        );
    }
}


