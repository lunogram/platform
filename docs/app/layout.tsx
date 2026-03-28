import { Inter } from "next/font/google";
import { Provider } from "@/components/provider";
import type { Metadata } from "next";
import "./global.css";

const inter = Inter({
  subsets: ["latin"],
});

export const metadata: Metadata = {
  title: {
    template: "%s | Lunogram",
    default: "Lunogram",
  },
  description: "Customer engagement, simplified",
  metadataBase: new URL("https://lunogram.com"),
  openGraph: {
    title: "Lunogram",
    description: "Customer engagement, simplified",
    siteName: "Lunogram",
    images: [
      {
        url: "/og-image.png",
        width: 1200,
        height: 630,
        type: "image/png",
      },
    ],
  },
  twitter: {
    card: "summary_large_image",
    title: "Lunogram",
    description: "Customer engagement, simplified",
    images: ["/og-image.png"],
  },
};

export default function Layout({ children }: LayoutProps<"/">) {
  return (
    <html lang="en" className={inter.className} suppressHydrationWarning>
      <body className="flex flex-col min-h-screen">
        <Provider>{children}</Provider>
      </body>
    </html>
  );
}
