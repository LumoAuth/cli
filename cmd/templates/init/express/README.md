# LumoAuth Express Starter

```bash
npm install
npm start
```

`/auth/login` → LumoAuth → `/auth/callback` → session cookie → `/`.
`req.user` populated for every request after sign-in.

Make sure your OAuth client (dashboard → Developers → OAuth Clients) has the
redirect URI `http://localhost:3000/auth/callback` and the grant types
`authorization_code,refresh_token`.
