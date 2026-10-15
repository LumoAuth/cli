# LumoAuth Express Starter

```bash
npm install
npm start
```

`/auth/login` → LumoAuth → `/auth/callback` → session cookie → `/`.
`req.user` populated for every request after sign-in.

OAuth client redirect URI: `http://localhost:3000/auth/callback` (already
allowed on the **Starter App** client). To use a client of your own, create it
in the portal under **Applications → New application** with that redirect URI
and the grant types `authorization_code,refresh_token`, then set
`LUMO_CLIENT_ID` (and `LUMO_CLIENT_SECRET` for a confidential client) in `.env`.
