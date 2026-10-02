import "dotenv/config";
import express from "express";
import session from "express-session";
import cookieParser from "cookie-parser";
import { lumoAuthMiddleware, requireAuth } from "@lumoauth/express";

const app = express();
app.use(cookieParser());
app.use(session({
  secret: process.env.SESSION_SECRET,
  resave: false,
  saveUninitialized: false,
  cookie: { httpOnly: true, sameSite: "lax", secure: process.env.NODE_ENV === "production" },
}));

// Wires GET /auth/login, GET /auth/callback, GET /auth/logout. Sets req.user
// from the session for downstream handlers.
app.use(lumoAuthMiddleware({
  baseUrl: process.env.LUMO_BASE_URL,
  organization: process.env.LUMO_ORG_ID,
  clientId: process.env.LUMO_CLIENT_ID,
  // Leave LUMO_CLIENT_SECRET empty for a public (PKCE-only) client such as
  // the Starter App.
  clientSecret: process.env.LUMO_CLIENT_SECRET || undefined,
}));

app.get("/", (req, res) => {
  if (!req.user) return res.redirect("/auth/login");
  res.send(`<h1>Hi ${req.user.email}</h1><a href="/auth/logout">Sign out</a>`);
});

// Protected route: requireAuth() rejects with 401 when no session.
app.get("/api/me", requireAuth(), (req, res) => res.json(req.user));

const port = process.env.PORT || 3000;
app.listen(port, () => console.log(`http://localhost:${port}`));
