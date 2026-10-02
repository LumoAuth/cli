# LumoAuth FastAPI Starter

```bash
python3 -m venv .venv && . .venv/bin/activate
pip install -r requirements.txt
uvicorn main:app --reload --port 3000
```

Then visit http://localhost:3000.

Sets up `/auth/login`, `/auth/callback`, `/auth/logout` via the LumoAuth
Python SDK's FastAPI router. The `get_current_user` dependency injects the
signed-in user into protected routes (`/api/me`).

OAuth client redirect URI: `http://localhost:3000/auth/callback` (already
allowed on the **Starter App** client). If you run on another port, register
that port's `/auth/callback` on the client too.
