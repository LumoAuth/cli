"use client";

import { AuthCallback } from "@lumoauth/react";

export default function CallbackPage() {
  return <AuthCallback afterSignInUrl="/" />;
}
