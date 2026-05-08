# LumoAuth FastAPI Starter

```bash
pip install -r requirements.txt
uvicorn main:app --reload
```

Sets up `/auth/login`, `/auth/callback`, `/auth/logout` via the LumoAuth
Python SDK's FastAPI router. The `get_current_user` dependency injects the
signed-in user into protected routes (`/api/me`).

OAuth client redirect URI: `http://localhost:8000/auth/callback`.
