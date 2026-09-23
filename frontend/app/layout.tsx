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
      {/*
        The dark text colour needs a dark-mode counterpart. Without one, every
        element that does not set its own colour inherits near-black in both
        themes -- which is invisible against a dark background. The `ghost`
        button variant is the main casualty: it only sets a colour on hover, so
        at rest it inherits, and its label disappeared entirely in dark mode
        across roughly thirty buttons app-wide.
      */}
      <body className="bg-gray-50 dark:bg-slate-950 min-h-screen text-gray-900 dark:text-slate-200 font-sans">
        <Providers>{children}</Providers>
      </body>
    </html>
  );
}
