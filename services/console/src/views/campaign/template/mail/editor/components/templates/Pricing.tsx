import type { ComponentConfig, Slot } from '@measured/puck';
import { Container as EmailContainer, Section as EmailSection } from '@react-email/components';
import { Layout, type LayoutProps, layoutClassMap } from '../fields/Layout';
import { cn } from '@/utils';
import { Spacing, type SpacingProps, spacingClassMap } from '../fields/Spacing';
import { Typography, type TypographyProps, typographyClassMap } from '../fields/Typography';
import { Decoration, type DecorationProps, decorationClassMap } from '../fields/Decoration';
import { generateTailwindClasses } from '../fields/unit';

export interface PricingProps {
    content: Slot;
    layout: LayoutProps;
    spacing: SpacingProps;
    typography: TypographyProps;
    decoration: DecorationProps;
};

export const Pricing: ComponentConfig<PricingProps> = {
    fields: {
        content: {
            type: "slot",
        },
        layout: Layout,
        typography: Typography,
        spacing: Spacing,
        decoration: Decoration,
    },
    defaultProps: {
        layout: {},
        typography: {},
        spacing: {},
        decoration: {},
        content: [
            {
                type: "Column",
                props: {
                    align: "center",
                    layout: {
                        xl: {
                            maxWidth: '90%',
                        }
                    },
                    spacing: {
                        xl: {
                            paddingTop: '28',
                            paddingBottom: '28',
                            paddingLeft: '28',
                            paddingRight: '28',
                            marginTop: '32',
                            marginBottom: '32',
                            marginLeft: 'auto',
                            marginRight: 'auto',
                            marginLinked: false
                        }
                    },
                    decoration: {
                        xl: {
                            backgroundColor: '#ffffff',
                            borderTopWidth: '1',
                            borderRightWidth: '1',
                            borderBottomWidth: '1',
                            borderLeftWidth: '1',
                            borderStyle: 'solid',
                            borderColor: '#d1d5db',
                            borderTopLeftRadius: '12',
                            borderTopRightRadius: '12',
                            borderBottomLeftRadius: '12',
                            borderBottomRightRadius: '12',
                        },
                    },
                    content: [
                        {
                            type: "Text",
                            props: {
                                typography: {
                                    xl: {
                                        fontSize: '12',
                                        fontWeight: 'semibold',
                                        color: '#4f46e5',
                                        lineHeight: '20',
                                        letterSpacing: 'wide',
                                        textTransform: 'uppercase',
                                    }
                                },
                                spacing: {
                                    xl: {
                                        marginTop: '16',
                                        marginBottom: '16',
                                    }
                                },
                                content: [
                                    {
                                        type: "TextSection",
                                        props: {
                                            value: "Exclusive Offer",
                                        },
                                    },
                                ],
                            },
                        },
                        {
                            type: "Text",
                            props: {
                                spacing: {
                                    xl: {
                                        marginTop: '0',
                                        marginBottom: '12',
                                    }
                                },
                                content: [
                                    {
                                        type: "TextSection",
                                        props: {
                                            value: "$12",
                                            typography: {
                                                xl: {
                                                    fontSize: '30',
                                                    fontWeight: 'bold',
                                                    color: '#101828',
                                                    lineHeight: '36',
                                                }
                                            },
                                        },
                                    },
                                    {
                                        type: "TextSection",
                                        props: {
                                            value: " / month",
                                            typography: {
                                                xl: {
                                                    fontSize: '16',
                                                    fontWeight: 'medium',
                                                    color: '#101828',
                                                    lineHeight: '20',
                                                }
                                            },
                                        },
                                    },
                                ],
                            },
                        },
                        {
                            type: "Text",
                            props: {
                                typography: {
                                    xl: {
                                        fontSize: '14',
                                        fontWeight: 'normal',
                                        color: '#374151',
                                        lineHeight: '20',
                                    }
                                },
                                spacing: {
                                    xl: {
                                        marginTop: '16',
                                        marginBottom: '24',
                                    }
                                },
                                content: [
                                    {
                                        type: "TextSection",
                                        props: {
                                            value: "We've handcrafted the perfect plan tailored specifically for your needs. Unlock premium features at an unbeatable value.",
                                        },
                                    },
                                ],
                            },
                        },
                        {
                            type: "Button",
                            props: {
                                value: "Claim Your Special Offer",
                                href: "#",
                                layout: {
                                    xl: {
                                        width: '100%',
                                    }
                                },
                                typography: {
                                    xl: {
                                        textAlign: 'center',
                                        fontSize: '16',
                                        fontWeight: 'bold',
                                        color: '#ffffff',
                                        lineHeight: '24',
                                        letterSpacing: 'wide',
                                    }
                                },
                                spacing: {
                                    xl: {
                                        paddingTop: '14',
                                        paddingBottom: '14',
                                        paddingLeft: '14',
                                        paddingRight: '14',
                                        marginBottom: '24',
                                    }
                                },
                                decoration: {
                                    xl: {
                                        borderTopLeftRadius: '8',
                                        borderTopRightRadius: '8',
                                        borderBottomLeftRadius: '8',
                                        borderBottomRightRadius: '8',
                                        backgroundColor: '#4f46e5',
                                    }
                                },
                            },
                        },
                        {
                            type: "Divider",
                            props: {
                                layout: {
                                    xl: {
                                        width: '100%',
                                    }
                                },
                                spacing: {},
                                decoration: {
                                    xl: {
                                        borderTopWidth: '1',
                                        borderWidthLinked: false,
                                        borderColor: '#d1d5db',
                                        borderStyle: 'solid',
                                    }
                                },
                            }
                        },
                        {
                            type: "Text",
                            props: {
                                typography: {
                                    xl: {
                                        fontSize: '12',
                                        fontWeight: 'normal',
                                        color: '#6b7280',
                                        lineHeight: '16',
                                        textAlign: 'center',
                                        fontStyle: 'italic',
                                    }
                                },
                                spacing: {
                                    xl: {
                                        marginTop: '24',
                                        marginBottom: '0',
                                    }
                                },
                                content: [
                                    {
                                        type: "TextSection",
                                        props: {
                                            value: "Limited time offer - Upgrade now and save 20%",
                                        },
                                    },
                                ],
                            },
                        },
                        {
                            type: "Text",
                            props: {
                                typography: {
                                    xl: {
                                        fontSize: '12',
                                        fontWeight: 'normal',
                                        color: '#6b7280',
                                        lineHeight: '16',
                                        textAlign: 'center',
                                    }
                                },
                                spacing: {
                                    xl: {
                                        marginTop: '0',
                                        marginBottom: '0',
                                    }
                                },
                                content: [
                                    {
                                        type: "TextSection",
                                        props: {
                                            value: "No credit card required. 14-day free trial available.",
                                        },
                                    },
                                ],
                            },
                        },
                    ],
                },
            },
        ],
    },
    render: ({ content: Content, layout, spacing, typography, decoration }) => {
        const classes = cn(
            generateTailwindClasses(layout, layoutClassMap),
            generateTailwindClasses(spacing, spacingClassMap),
            generateTailwindClasses(typography, typographyClassMap),
            generateTailwindClasses(decoration, decorationClassMap),
        )

        return (
            <EmailSection className={classes}>
                <EmailContainer style={{ margin: '0 auto' }}>
                    <Content />
                </EmailContainer>
            </EmailSection>
        );
    },
}
