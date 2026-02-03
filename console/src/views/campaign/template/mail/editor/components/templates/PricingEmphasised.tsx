// 1. Updated import source
import type { ComponentConfig, Slot } from '@puckeditor/core'; 
import { Container as EmailContainer, Section as EmailSection } from '@react-email/components';
import { Layout, type LayoutProps, layoutClassMap } from '../fields/Layout';
import { cn } from '@/utils';
import { Spacing, type SpacingProps, spacingClassMap } from '../fields/Spacing';
import { Typography, type TypographyProps, typographyClassMap } from '../fields/Typography';
import { Decoration, type DecorationProps, decorationClassMap } from '../fields/Decoration';
import { generateTailwindClasses } from '../fields/unit';

export interface PricingEmphasisedProps {
    content: Slot;
    layout: LayoutProps;
    spacing: SpacingProps;
    typography: TypographyProps;
    decoration: DecorationProps;
};

export const PricingEmphasised: ComponentConfig<PricingEmphasisedProps> = {
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
        spacing: {
            xl: {
                paddingTop: '24',
                paddingBottom: '24',
                paddingLeft: '24',
                paddingRight: '24',
            }
        },
        typography: {},
        decoration: {
            xl: {
                backgroundColor: '#ffffff',
                borderTopLeftRadius: '8',
                borderTopRightRadius: '8',
                borderBottomLeftRadius: '8',
                borderBottomRightRadius: '8',
            }
        },
        content: [
            // Header Section
            {
                type: "Container",
                props: {
                    spacing: {
                        xl: {
                            marginLinked: false,
                            marginLeft: 'auto',
                            marginRight: 'auto',
                            marginBottom: '0',
                        }
                    },
                    content: [
                        {
                            type: "Text",
                            props: {
                                typography: {
                                    xl: {
                                        fontSize: '24',
                                        fontWeight: 'semibold',
                                        textAlign: 'center',
                                    }
                                },
                                spacing: {
                                    xl: {
                                        marginBottom: '12',
                                    }
                                },
                                content: [
                                    {
                                        type: "TextSection",
                                        props: {
                                            value: "Choose the right plan for you",
                                        }
                                    }
                                ]
                            }
                        },
                        {
                            type: "Text",
                            props: {
                                layout: {},
                                typography: {
                                    xl: {
                                        fontSize: '14',
                                        textAlign: 'center',
                                        color: '#6b7280',
                                    }
                                },
                                content: [
                                    {
                                        type: "TextSection",
                                        props: {
                                            value: "Choose an affordable plan with top features to engage audiences, build loyalty, and boost sales.",
                                        }
                                    }
                                ]
                            }
                        }
                    ]
                }
            },
            // Pricing Plans Section
            {
                type: "Container",
                props: {
                    spacing: {
                        xl: {
                            paddingBottom: '24',
                        }
                    },
                    content: [
                        // Hobby Plan
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
                                        paddingTop: '24',
                                        paddingBottom: '24',
                                        paddingLeft: '24',
                                        paddingRight: '24',
                                        marginTop: '32',
                                        marginBottom: '24',
                                        marginLeft: 'auto',
                                        marginRight: 'auto',
                                        marginLinked: false,
                                    }
                                },
                                decoration: {
                                    xl: {
                                        backgroundColor: '#ffffff',
                                        borderTopWidth: '1',
                                        borderBottomWidth: '1',
                                        borderLeftWidth: '1',
                                        borderRightWidth: '1',
                                        borderWidthLinked: true,
                                        borderColor: '#d1d5db',
                                        borderStyle: 'solid',
                                        borderTopLeftRadius: '8',
                                        borderTopRightRadius: '8',
                                        borderBottomLeftRadius: '8',
                                        borderBottomRightRadius: '8',
                                    }
                                },
                                content: [
                                    {
                                        type: "Text",
                                        props: {
                                            typography: {
                                                xl: {
                                                    fontSize: '14',
                                                    fontWeight: 'semibold',
                                                    color: '#4f46e5',
                                                }
                                            },
                                            spacing: {
                                                xl: {
                                                    marginBottom: '16',
                                                }
                                            },
                                            content: [
                                                {
                                                    type: "TextSection",
                                                    props: {
                                                        value: "Hobby",
                                                    }
                                                }
                                            ]
                                        }
                                    },
                                    {
                                        type: "Text",
                                        props: {
                                            typography: {
                                                xl: {
                                                    fontSize: '28',
                                                    fontWeight: 'bold',
                                                }
                                            },
                                            spacing: {
                                                xl: {
                                                    marginTop: '0',
                                                    marginBottom: '8',
                                                }
                                            },
                                            content: [
                                                {
                                                    type: "TextSection",
                                                    props: {
                                                        value: "$29",
                                                        typography: {
                                                            xl: {
                                                                color: '#101828',
                                                            }
                                                        }
                                                    }
                                                },
                                                {
                                                    type: "TextSection",
                                                    props: {
                                                        value: " / month",
                                                        typography: {
                                                            xl: {
                                                                fontSize: '14',
                                                                color: '#6b7280',
                                                            }
                                                        }
                                                    }
                                                }
                                            ]
                                        }
                                    },
                                    {
                                        type: "Text",
                                        props: {
                                            typography: {
                                                xl: {
                                                    color: '#6b7280',
                                                }
                                            },
                                            spacing: {
                                                xl: {
                                                    marginTop: '12',
                                                    marginBottom: '24',
                                                }
                                            },
                                            content: [
                                                {
                                                    type: "TextSection",
                                                    props: {
                                                        value: "The perfect plan for getting started.",
                                                    }
                                                }
                                            ]
                                        }
                                    },
                                    {
                                        type: "Button",
                                        props: {
                                            value: "Get started today",
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
                                                    fontWeight: 'semibold',
                                                    color: '#ffffff',
                                                }
                                            },
                                            spacing: {},
                                            decoration: {
                                                xl: {
                                                    borderTopLeftRadius: '8',
                                                    borderTopRightRadius: '8',
                                                    borderBottomLeftRadius: '8',
                                                    borderBottomRightRadius: '8',
                                                    backgroundColor: '#4f46e5',
                                                }
                                            },
                                        }
                                    }
                                ]
                            }
                        },
                        // Enterprise Plan (Highlighted)
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
                                        paddingTop: '24',
                                        paddingBottom: '24',
                                        paddingLeft: '24',
                                        paddingRight: '24',
                                        marginBottom: '12',
                                        marginLeft: 'auto',
                                        marginRight: 'auto',
                                        marginLinked: false,
                                    }
                                },
                                decoration: {
                                    xl: {
                                        backgroundColor: '#101828',
                                        borderTopWidth: '1',
                                        borderBottomWidth: '1',
                                        borderLeftWidth: '1',
                                        borderRightWidth: '1',
                                        borderWidthLinked: true,
                                        borderColor: '#101828',
                                        borderStyle: 'solid',
                                        borderTopLeftRadius: '8',
                                        borderTopRightRadius: '8',
                                        borderBottomLeftRadius: '8',
                                        borderBottomRightRadius: '8',
                                    }
                                },
                                content: [
                                    {
                                        type: "Text",
                                        props: {
                                            typography: {
                                                xl: {
                                                    fontSize: '14',
                                                    fontWeight: 'semibold',
                                                    color: '#7c86ff',
                                                }
                                            },
                                            spacing: {
                                                xl: {
                                                    marginBottom: '16',
                                                }
                                            },
                                            content: [
                                                {
                                                    type: "TextSection",
                                                    props: {
                                                        value: "Enterprise",
                                                    }
                                                }
                                            ]
                                        }
                                    },
                                    {
                                        type: "Text",
                                        props: {
                                            typography: {
                                                xl: {
                                                    fontSize: '28',
                                                    fontWeight: 'bold',
                                                }
                                            },
                                            spacing: {
                                                xl: {
                                                    marginTop: '0',
                                                    marginBottom: '8',
                                                }
                                            },
                                            content: [
                                                {
                                                    type: "TextSection",
                                                    props: {
                                                        value: "$99",
                                                        typography: {
                                                            xl: {
                                                                color: '#ffffff',
                                                            }
                                                        }
                                                    }
                                                },
                                                {
                                                    type: "TextSection",
                                                    props: {
                                                        value: " / month",
                                                        typography: {
                                                            xl: {
                                                                fontSize: '14',
                                                                color: '#d1d5db',
                                                            }
                                                        }
                                                    }
                                                }
                                            ]
                                        }
                                    },
                                    {
                                        type: "Text",
                                        props: {
                                            typography: {
                                                xl: {
                                                    color: '#d1d5db',
                                                }
                                            },
                                            spacing: {
                                                xl: {
                                                    marginTop: '12',
                                                    marginBottom: '24',
                                                }
                                            },
                                            content: [
                                                {
                                                    type: "TextSection",
                                                    props: {
                                                        value: "Dedicated support and enterprise ready.",
                                                    }
                                                }
                                            ]
                                        }
                                    },
                                    {
                                        type: "Button",
                                        props: {
                                            value: "Get started today",
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
                                                    fontWeight: 'semibold',
                                                    color: '#ffffff',
                                                }
                                            },
                                            spacing: {},
                                            decoration: {
                                                xl: {
                                                    borderTopLeftRadius: '8',
                                                    borderTopRightRadius: '8',
                                                    borderBottomLeftRadius: '8',
                                                    borderBottomRightRadius: '8',
                                                    backgroundColor: '#4f46e5',
                                                }
                                            },
                                        }
                                    }
                                ]
                            }
                        }
                    ]
                }
            },
            // Divider
            {
                type: "Divider",
                props: {
                    layout: {
                        xl: {
                            width: '100%',
                        }
                    },
                    spacing: {
                        xl: {
                            marginTop: '0',
                            marginBottom: '16',
                        }
                    },
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
            // Footer
            {
                type: "Text",
                props: {
                    typography: {
                        xl: {
                            fontSize: '12',
                            fontWeight: 'medium',
                            textAlign: 'center',
                            color: '#6b7280',
                        }
                    },
                    spacing: {
                        xl: {
                            marginTop: '32',
                            marginBottom: '32',
                        }
                    },
                    content: [
                        {
                            type: "TextSection",
                            props: {
                                value: "Customer Experience Research Team",
                            }
                        }
                    ]
                }
            }
        ]
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