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
    <html lang="en" className={outfit.variable}>
      <body className="bg-gray-50 min-h-screen text-gray-900 font-sans">
        <Providers>{children}</Providers>
      </body>
    </html>
  );
}
