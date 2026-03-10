import { Render, type Data } from "@puckeditor/core"
import { Body, Font, Head, Html, pixelBasedPreset, render, Tailwind } from "@react-email/components"
import parse from "html-react-parser"
import { renderToString } from "react-dom/server"
import { config } from "./ConfigHandler"

export async function renderBlockToHtml(data: Data, locale = "en") {
    const content = renderToString(<Render config={config} data={data} />)

    const tailwindConfig = {
        presets: [pixelBasedPreset],
    }

    return await render(
        <Html lang={locale}>
            <Head>
                <Font
                    fontFamily="Roboto"
                    fallbackFontFamily="Verdana"
                    webFont={{
                        url: "https://fonts.gstatic.com/s/roboto/v27/KFOmCnqEu92Fr1Mu4mxKKTU1Kg.woff2",
                        format: "woff2",
                    }}
                    fontWeight={400}
                    fontStyle="normal"
                />
            </Head>
            <Tailwind config={tailwindConfig}>
                <Body>{parse(content)}</Body>
            </Tailwind>
        </Html>,
    )
}
