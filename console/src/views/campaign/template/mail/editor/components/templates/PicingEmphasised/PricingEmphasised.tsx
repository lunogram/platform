// 1. Updated import source
import type { ComponentConfig, Slot } from "@puckeditor/core";
import {
  Container as EmailContainer,
  Section as EmailSection,
} from "@react-email/components";
import { Layout, type LayoutProps, layoutClassMap } from "../../fields/Layout";
import { cn } from "@/utils";
import {
  Spacing,
  type SpacingProps,
  spacingClassMap,
} from "../../fields/Spacing";
import {
  Typography,
  type TypographyProps,
  typographyClassMap,
} from "../../fields/Typography";
import {
  Decoration,
  type DecorationProps,
  decorationClassMap,
} from "../../fields/Decoration";
import { generateTailwindClasses } from "../../fields/unit";

export interface PricingEmphasisedProps {
  content: Slot;
  layout: LayoutProps;
  spacing: SpacingProps;
  typography: TypographyProps;
  decoration: DecorationProps;
}

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
        paddingTop: "24",
        paddingBottom: "24",
        paddingLeft: "24",
        paddingRight: "24",
      },
    },
    typography: {},
    decoration: {
      xl: {
        backgroundColor: "#ffffff",
        borderTopLeftRadius: "8",
        borderTopRightRadius: "8",
        borderBottomLeftRadius: "8",
        borderBottomRightRadius: "8",
      },
    },
    content: [
      // Header Section
      {
        type: "Container",
        props: {
          spacing: {
            xl: {
              marginLinked: false,
              marginLeft: "auto",
              marginRight: "auto",
              marginBottom: "0",
            },
          },
          content: [
            {
              type: "Text",
              props: {
                typography: {
                  xl: {
                    fontSize: "24",
                    fontWeight: "semibold",
                    textAlign: "center",
                  },
                },
                spacing: {
                  xl: {
                    marginBottom: "12",
                  },
                },
                content: [
                  {
                    type: "TextSection",
                    props: {
                      value: "Choose the right plan for you",
                    },
                  },
                ],
              },
            },
            {
              type: "Text",
              props: {
                layout: {},
                typography: {
                  xl: {
                    fontSize: "14",
                    textAlign: "center",
                    color: "#6b7280",
                  },
                },
                content: [
                  {
                    type: "TextSection",
                    props: {
                      value:
                        "Choose an affordable plan with top features to engage audiences, build loyalty, and boost sales.",
                    },
                  },
                ],
              },
            },
          ],
        },
      },
      // Pricing Plans Section
      {
        type: "Container",
        props: {
          spacing: {
            xl: {
              paddingBottom: "24",
            },
          },
          content: [
            // Hobby Plan
            {
              type: "Column",
              props: {
                align: "center",
                layout: {
                  xl: {
                    maxWidth: "90%",
                  },
                },
                spacing: {
                  xl: {
                    paddingTop: "24",
                    paddingBottom: "24",
                    paddingLeft: "24",
                    paddingRight: "24",
                    marginTop: "32",
                    marginBottom: "24",
                    marginLeft: "auto",
                    marginRight: "auto",
                    marginLinked: false,
                  },
                },
                decoration: {
                  xl: {
                    backgroundColor: "#ffffff",
                    borderTopWidth: "1",
                    borderBottomWidth: "1",
                    borderLeftWidth: "1",
                    borderRightWidth: "1",
                    borderWidthLinked: true,
                    borderColor: "#d1d5db",
                    borderStyle: "solid",
                    borderTopLeftRadius: "8",
                    borderTopRightRadius: "8",
                    borderBottomLeftRadius: "8",
                    borderBottomRightRadius: "8",
                  },
                },
                content: [
                  {
                    type: "Text",
                    props: {
                      typography: {
                        xl: {
                          fontSize: "14",
                          fontWeight: "semibold",
                          color: "#4f46e5",
                        },
                      },
                      spacing: {
                        xl: {
                          marginBottom: "16",
                        },
                      },
                      content: [
                        {
                          type: "TextSection",
                          props: {
                            value: "Hobby",
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
                          fontSize: "28",
                          fontWeight: "bold",
                        },
                      },
                      spacing: {
                        xl: {
                          marginTop: "0",
                          marginBottom: "8",
                        },
                      },
                      content: [
                        {
                          type: "TextSection",
                          props: {
                            value: "$29",
                            typography: {
                              xl: {
                                color: "#101828",
                              },
                            },
                          },
                        },
                        {
                          type: "TextSection",
                          props: {
                            value: " / month",
                            typography: {
                              xl: {
                                fontSize: "14",
                                color: "#6b7280",
                              },
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
                          color: "#6b7280",
                        },
                      },
                      spacing: {
                        xl: {
                          marginTop: "12",
                          marginBottom: "24",
                        },
                      },
                      content: [
                        {
                          type: "TextSection",
                          props: {
                            value: "The perfect plan for getting started.",
                          },
                        },
                      ],
                    },
                  },
                  {
                    type: "Button",
                    props: {
                      value: "Get started today",
                      href: "#",
                      layout: {
                        xl: {
                          width: "100%",
                        },
                      },
                      typography: {
                        xl: {
                          textAlign: "center",
                          fontSize: "16",
                          fontWeight: "semibold",
                          color: "#ffffff",
                        },
                      },
                      spacing: {},
                      decoration: {
                        xl: {
                          borderTopLeftRadius: "8",
                          borderTopRightRadius: "8",
                          borderBottomLeftRadius: "8",
                          borderBottomRightRadius: "8",
                          backgroundColor: "#4f46e5",
                        },
                      },
                    },
                  },
                ],
              },
            },
            // Enterprise Plan (Highlighted)
            {
              type: "Column",
              props: {
                align: "center",
                layout: {
                  xl: {
                    maxWidth: "90%",
                  },
                },
                spacing: {
                  xl: {
                    paddingTop: "24",
                    paddingBottom: "24",
                    paddingLeft: "24",
                    paddingRight: "24",
                    marginBottom: "12",
                    marginLeft: "auto",
                    marginRight: "auto",
                    marginLinked: false,
                  },
                },
                decoration: {
                  xl: {
                    backgroundColor: "#101828",
                    borderTopWidth: "1",
                    borderBottomWidth: "1",
                    borderLeftWidth: "1",
                    borderRightWidth: "1",
                    borderWidthLinked: true,
                    borderColor: "#101828",
                    borderStyle: "solid",
                    borderTopLeftRadius: "8",
                    borderTopRightRadius: "8",
                    borderBottomLeftRadius: "8",
                    borderBottomRightRadius: "8",
                  },
                },
                content: [
                  {
                    type: "Text",
                    props: {
                      typography: {
                        xl: {
                          fontSize: "14",
                          fontWeight: "semibold",
                          color: "#7c86ff",
                        },
                      },
                      spacing: {
                        xl: {
                          marginBottom: "16",
                        },
                      },
                      content: [
                        {
                          type: "TextSection",
                          props: {
                            value: "Enterprise",
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
                          fontSize: "28",
                          fontWeight: "bold",
                        },
                      },
                      spacing: {
                        xl: {
                          marginTop: "0",
                          marginBottom: "8",
                        },
                      },
                      content: [
                        {
                          type: "TextSection",
                          props: {
                            value: "$99",
                            typography: {
                              xl: {
                                color: "#ffffff",
                              },
                            },
                          },
                        },
                        {
                          type: "TextSection",
                          props: {
                            value: " / month",
                            typography: {
                              xl: {
                                fontSize: "14",
                                color: "#d1d5db",
                              },
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
                          color: "#d1d5db",
                        },
                      },
                      spacing: {
                        xl: {
                          marginTop: "12",
                          marginBottom: "24",
                        },
                      },
                      content: [
                        {
                          type: "TextSection",
                          props: {
                            value: "Dedicated support and enterprise ready.",
                          },
                        },
                      ],
                    },
                  },
                  {
                    type: "Button",
                    props: {
                      value: "Get started today",
                      href: "#",
                      layout: {
                        xl: {
                          width: "100%",
                        },
                      },
                      typography: {
                        xl: {
                          textAlign: "center",
                          fontSize: "16",
                          fontWeight: "semibold",
                          color: "#ffffff",
                        },
                      },
                      spacing: {},
                      decoration: {
                        xl: {
                          borderTopLeftRadius: "8",
                          borderTopRightRadius: "8",
                          borderBottomLeftRadius: "8",
                          borderBottomRightRadius: "8",
                          backgroundColor: "#4f46e5",
                        },
                      },
                    },
                  },
                ],
              },
            },
          ],
        },
      },
      // Divider
      {
        type: "Divider",
        props: {
          layout: {
            xl: {
              width: "100%",
            },
          },
          spacing: {
            xl: {
              marginTop: "0",
              marginBottom: "16",
            },
          },
          decoration: {
            xl: {
              borderTopWidth: "1",
              borderWidthLinked: false,
              borderColor: "#d1d5db",
              borderStyle: "solid",
            },
          },
        },
      },
      // Footer
      {
        type: "Text",
        props: {
          typography: {
            xl: {
              fontSize: "12",
              fontWeight: "medium",
              textAlign: "center",
              color: "#6b7280",
            },
          },
          spacing: {
            xl: {
              marginTop: "32",
              marginBottom: "32",
            },
          },
          content: [
            {
              type: "TextSection",
              props: {
                value: "Customer Experience Research Team",
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
    );

    return (
      <EmailSection className={classes}>
        <EmailContainer style={{ margin: "0 auto" }}>
          <Content />
        </EmailContainer>
      </EmailSection>
    );
  },
};

export const PricingEmphasisedTemplate = {
  root: {
    props: {},
  },
  zones: {},
  content: [
    {
      type: "PricingEmphasised",
      props: {
        id: "PricingEmphasised-2f7dac83-5875-42e2-a62c-7883caa9fc59",
        layout: {},
        content: [
          {
            type: "Container",
            props: {
              id: "Container-3d6d95a2-3494-4c88-b42d-e413a68cdc31",
              content: [
                {
                  type: "Text",
                  props: {
                    id: "Text-b3e7b6e9-8339-42f6-ae1b-44d7d656c339",
                    content: [
                      {
                        type: "TextSection",
                        props: {
                          id: "TextSection-9809e0bc-d1ce-4866-8e6d-1663fb3c0be6",
                          value: "Choose the right plan for you",
                        },
                      },
                    ],
                    spacing: {
                      xl: {
                        marginBottom: "12",
                      },
                    },
                    typography: {
                      xl: {
                        fontSize: "24",
                        textAlign: "center",
                        fontWeight: "semibold",
                      },
                    },
                  },
                },
                {
                  type: "Text",
                  props: {
                    id: "Text-a8a7a8c8-d241-4d0f-a58b-dabc298b5d7e",
                    layout: {},
                    content: [
                      {
                        type: "TextSection",
                        props: {
                          id: "TextSection-22bacbd7-6b17-4d45-b55b-efcd83fa1a91",
                          value:
                            "Choose an affordable plan with top features to engage audiences, build loyalty, and boost sales.",
                        },
                      },
                    ],
                    typography: {
                      xl: {
                        color: "#6b7280",
                        fontSize: "14",
                        textAlign: "center",
                      },
                    },
                  },
                },
              ],
              spacing: {
                xl: {
                  marginLeft: "auto",
                  marginRight: "auto",
                  marginBottom: "0",
                  marginLinked: false,
                },
              },
            },
          },
          {
            type: "Container",
            props: {
              id: "Container-912eb287-2fb2-496e-ab21-c55810c37773",
              content: [
                {
                  type: "Column",
                  props: {
                    id: "Column-398ac3ff-b3a6-49e2-b13b-c9c04415773c",
                    align: "center",
                    layout: {
                      xl: {
                        maxWidth: "90%",
                      },
                    },
                    content: [
                      {
                        type: "Text",
                        props: {
                          id: "Text-82f6f195-9526-476c-b43b-d18b0272a449",
                          content: [
                            {
                              type: "TextSection",
                              props: {
                                id: "TextSection-cf3600dd-96f2-47a3-a076-a827966deeaa",
                                value: "Hobby",
                              },
                            },
                          ],
                          spacing: {
                            xl: {
                              marginBottom: "16",
                            },
                          },
                          typography: {
                            xl: {
                              color: "#4f46e5",
                              fontSize: "14",
                              fontWeight: "semibold",
                            },
                          },
                        },
                      },
                      {
                        type: "Text",
                        props: {
                          id: "Text-fb294dbb-1405-4a46-9cd2-b6fe989628c3",
                          content: [
                            {
                              type: "TextSection",
                              props: {
                                id: "TextSection-62af84b6-8342-449b-8c64-9a7c9c741fda",
                                value: "$29",
                                typography: {
                                  xl: {
                                    color: "#101828",
                                  },
                                },
                              },
                            },
                            {
                              type: "TextSection",
                              props: {
                                id: "TextSection-5db1a03a-e027-4dd9-b000-2c2d0d396622",
                                value: " / month",
                                typography: {
                                  xl: {
                                    color: "#6b7280",
                                    fontSize: "14",
                                  },
                                },
                              },
                            },
                          ],
                          spacing: {
                            xl: {
                              marginTop: "0",
                              marginBottom: "8",
                            },
                          },
                          typography: {
                            xl: {
                              fontSize: "28",
                              fontWeight: "bold",
                            },
                          },
                        },
                      },
                      {
                        type: "Text",
                        props: {
                          id: "Text-c2ce0698-9f7c-4714-8ca4-41df2d0cdcb7",
                          content: [
                            {
                              type: "TextSection",
                              props: {
                                id: "TextSection-dc701a91-83ae-48aa-8c80-09b663e9563a",
                                value: "The perfect plan for getting started.",
                              },
                            },
                          ],
                          spacing: {
                            xl: {
                              marginTop: "12",
                              marginBottom: "24",
                            },
                          },
                          typography: {
                            xl: {
                              color: "#6b7280",
                            },
                          },
                        },
                      },
                      {
                        type: "Button",
                        props: {
                          id: "Button-13e294b7-eae2-4cdb-a730-0ae70a4864bc",
                          href: "#",
                          value: "Get started today",
                          layout: {
                            xl: {
                              width: "100%",
                            },
                          },
                          spacing: {},
                          decoration: {
                            xl: {
                              backgroundColor: "#4f46e5",
                              borderTopLeftRadius: "8",
                              borderTopRightRadius: "8",
                              borderBottomLeftRadius: "8",
                              borderBottomRightRadius: "8",
                            },
                          },
                          typography: {
                            xl: {
                              color: "#ffffff",
                              fontSize: "16",
                              textAlign: "center",
                              fontWeight: "semibold",
                            },
                          },
                        },
                      },
                    ],
                    spacing: {
                      xl: {
                        marginTop: "32",
                        marginLeft: "auto",
                        paddingTop: "24",
                        marginRight: "auto",
                        paddingLeft: "24",
                        marginBottom: "24",
                        marginLinked: false,
                        paddingRight: "24",
                        paddingBottom: "24",
                      },
                    },
                    decoration: {
                      xl: {
                        borderColor: "#d1d5db",
                        borderStyle: "solid",
                        borderTopWidth: "1",
                        backgroundColor: "#ffffff",
                        borderLeftWidth: "1",
                        borderRightWidth: "1",
                        borderBottomWidth: "1",
                        borderWidthLinked: true,
                        borderTopLeftRadius: "8",
                        borderTopRightRadius: "8",
                        borderBottomLeftRadius: "8",
                        borderBottomRightRadius: "8",
                      },
                    },
                  },
                },
                {
                  type: "Column",
                  props: {
                    id: "Column-c97d94b3-7b18-4232-8d23-625dafad45a7",
                    align: "center",
                    layout: {
                      xl: {
                        maxWidth: "90%",
                      },
                    },
                    content: [
                      {
                        type: "Text",
                        props: {
                          id: "Text-0c8fe6dc-4d05-440d-97da-899e449960e8",
                          content: [
                            {
                              type: "TextSection",
                              props: {
                                id: "TextSection-caa1df7b-0679-4268-b7aa-d97a446f65f9",
                                value: "Enterprise",
                              },
                            },
                          ],
                          spacing: {
                            xl: {
                              marginBottom: "16",
                            },
                          },
                          typography: {
                            xl: {
                              color: "#7c86ff",
                              fontSize: "14",
                              fontWeight: "semibold",
                            },
                          },
                        },
                      },
                      {
                        type: "Text",
                        props: {
                          id: "Text-32cfd397-04bc-4cef-bf6b-b2b291d64319",
                          content: [
                            {
                              type: "TextSection",
                              props: {
                                id: "TextSection-d96a5145-f596-48b1-915a-2f3570283c0f",
                                value: "$99",
                                typography: {
                                  xl: {
                                    color: "#ffffff",
                                  },
                                },
                              },
                            },
                            {
                              type: "TextSection",
                              props: {
                                id: "TextSection-d04c42a3-a3f3-46b9-81e0-05f4a66379fc",
                                value: " / month",
                                typography: {
                                  xl: {
                                    color: "#d1d5db",
                                    fontSize: "14",
                                  },
                                },
                              },
                            },
                          ],
                          spacing: {
                            xl: {
                              marginTop: "0",
                              marginBottom: "8",
                            },
                          },
                          typography: {
                            xl: {
                              fontSize: "28",
                              fontWeight: "bold",
                            },
                          },
                        },
                      },
                      {
                        type: "Text",
                        props: {
                          id: "Text-0118a8d7-a0b8-4692-85ea-7ac1afc1aea5",
                          content: [
                            {
                              type: "TextSection",
                              props: {
                                id: "TextSection-987928d5-1340-49f9-ab47-15a32304b008",
                                value:
                                  "Dedicated support and enterprise ready.",
                              },
                            },
                          ],
                          spacing: {
                            xl: {
                              marginTop: "12",
                              marginBottom: "24",
                            },
                          },
                          typography: {
                            xl: {
                              color: "#d1d5db",
                            },
                          },
                        },
                      },
                      {
                        type: "Button",
                        props: {
                          id: "Button-895242f2-ce9c-44a0-9b67-06b51b085069",
                          href: "#",
                          value: "Get started today",
                          layout: {
                            xl: {
                              width: "100%",
                            },
                          },
                          spacing: {},
                          decoration: {
                            xl: {
                              backgroundColor: "#4f46e5",
                              borderTopLeftRadius: "8",
                              borderTopRightRadius: "8",
                              borderBottomLeftRadius: "8",
                              borderBottomRightRadius: "8",
                            },
                          },
                          typography: {
                            xl: {
                              color: "#ffffff",
                              fontSize: "16",
                              textAlign: "center",
                              fontWeight: "semibold",
                            },
                          },
                        },
                      },
                    ],
                    spacing: {
                      xl: {
                        marginLeft: "auto",
                        paddingTop: "24",
                        marginRight: "auto",
                        paddingLeft: "24",
                        marginBottom: "12",
                        marginLinked: false,
                        paddingRight: "24",
                        paddingBottom: "24",
                      },
                    },
                    decoration: {
                      xl: {
                        borderColor: "#101828",
                        borderStyle: "solid",
                        borderTopWidth: "1",
                        backgroundColor: "#101828",
                        borderLeftWidth: "1",
                        borderRightWidth: "1",
                        borderBottomWidth: "1",
                        borderWidthLinked: true,
                        borderTopLeftRadius: "8",
                        borderTopRightRadius: "8",
                        borderBottomLeftRadius: "8",
                        borderBottomRightRadius: "8",
                      },
                    },
                  },
                },
              ],
              spacing: {
                xl: {
                  paddingBottom: "24",
                },
              },
            },
          },
          {
            type: "Divider",
            props: {
              id: "Divider-584e6eb5-e115-4e0d-b2bb-56fd47effe47",
              layout: {
                xl: {
                  width: "100%",
                },
              },
              spacing: {
                xl: {
                  marginTop: "0",
                  marginBottom: "16",
                },
              },
              decoration: {
                xl: {
                  borderColor: "#d1d5db",
                  borderStyle: "solid",
                  borderTopWidth: "1",
                  borderWidthLinked: false,
                },
              },
            },
          },
          {
            type: "Text",
            props: {
              id: "Text-44d64d00-6d1f-468e-8136-ba967bf3364d",
              content: [
                {
                  type: "TextSection",
                  props: {
                    id: "TextSection-1b20e374-d179-4758-aa9f-026176200c38",
                    value: "Customer Experience Research Team",
                  },
                },
              ],
              spacing: {
                xl: {
                  marginTop: "32",
                  marginBottom: "32",
                },
              },
              typography: {
                xl: {
                  color: "#6b7280",
                  fontSize: "12",
                  textAlign: "center",
                  fontWeight: "medium",
                },
              },
            },
          },
        ],
        spacing: {
          xl: {
            paddingTop: "24",
            paddingLeft: "24",
            paddingRight: "24",
            paddingBottom: "24",
          },
        },
        decoration: {
          xl: {
            backgroundColor: "#ffffff",
            borderTopLeftRadius: "8",
            borderTopRightRadius: "8",
            borderBottomLeftRadius: "8",
            borderBottomRightRadius: "8",
          },
        },
        typography: {},
      },
    },
  ],
};
