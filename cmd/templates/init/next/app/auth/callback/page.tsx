"use client";

import { AuthCallback } from "@lumoauth/nextjs";

export default function CallbackPage() {
  return <AuthCallback afterSignInUrl="/" />;
}
