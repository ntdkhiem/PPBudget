import "./globals.css";
import { Providers } from "./providers";
import { Outfit } from "next/font/google";

const outfit = Outfit({ subsets: ["latin"], variable: "--font-sans" });

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    // suppressHydrationWarning is the documented next-themes setup: the theme
    // provider sets the `dark` class and color-scheme on <html> after SSR, so
    // server and client markup necessarily differ on this one element.
    <html lang="en" className={outfit.variable} suppressHydrationWarning>
      <body className="bg-gray-50 min-h-screen text-gray-900 font-sans">
        <Providers>{children}</Providers>
      </body>
    </html>
  );
}
