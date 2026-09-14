import { LumoAuthProvider } from "@lumoauth/nextjs";
import type { ReactNode } from "react";

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="en">
      <body>
        <LumoAuthProvider
          domain={process.env.NEXT_PUBLIC_LUMOAUTH_DOMAIN!}
          orgId={process.env.NEXT_PUBLIC_LUMOAUTH_ORG_ID!}
          clientId={process.env.NEXT_PUBLIC_LUMOAUTH_CLIENT_ID!}
        >
          {children}
        </LumoAuthProvider>
      </body>
    </html>
  );
}
