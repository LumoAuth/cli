"use client";

import { SignIn, UserButton, useLumoAuth } from "@lumoauth/react";

export default function Home() {
  const { isAuthenticated, user } = useLumoAuth();

  if (!isAuthenticated) {
    return (
      <main style={{ padding: "4rem", maxWidth: 480, margin: "0 auto" }}>
        <h1>LumoAuth Starter</h1>
        <p>Sign in to continue.</p>
        <SignIn />
      </main>
    );
  }

  return (
    <main style={{ padding: "4rem", maxWidth: 480, margin: "0 auto" }}>
      <header style={{ display: "flex", justifyContent: "space-between" }}>
        <h1>Welcome, {user?.name ?? user?.email}</h1>
        <UserButton />
      </header>
      <p>You're authenticated. Build something here.</p>
    </main>
  );
}
