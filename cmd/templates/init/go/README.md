# LumoAuth Go Starter

```bash
go mod tidy
go run .
```

Visit http://localhost:3000. `/auth/login` redirects to LumoAuth; on return
your session cookie is set and `lumoauth.UserFromContext(ctx)` resolves to
the signed-in user.

OAuth client redirect URI: `http://localhost:3000/auth/callback`.
