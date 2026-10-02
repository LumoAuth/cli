"""LumoAuth + FastAPI starter.

Run:  uvicorn main:app --reload --port 3000
Visit http://localhost:3000 — you'll be redirected to LumoAuth to sign in.
"""
import os

from dotenv import load_dotenv
from fastapi import FastAPI, Depends, Request
from fastapi.responses import HTMLResponse, RedirectResponse
from starlette.middleware.sessions import SessionMiddleware

from lumoauth.fastapi import lumo_auth_router, get_current_user, require_auth, User

load_dotenv()

app = FastAPI()

app.add_middleware(SessionMiddleware, secret_key=os.environ["SESSION_SECRET"])

# Mounts /auth/login, /auth/callback, /auth/logout under the prefix.
app.include_router(
    lumo_auth_router(
        base_url=os.environ["LUMO_BASE_URL"],
        organization=os.environ["LUMO_ORG_ID"],
        client_id=os.environ["LUMO_CLIENT_ID"],
        # Leave LUMO_CLIENT_SECRET empty for a public (PKCE-only) client such
        # as the Starter App.
        client_secret=os.environ.get("LUMO_CLIENT_SECRET") or None,
        callback_path="/auth/callback",
    ),
    prefix="/auth",
)


@app.get("/", response_class=HTMLResponse)
def root(request: Request, user: User | None = Depends(get_current_user)):
    if user is None:
        return RedirectResponse("/auth/login")
    return f"<h1>Hi {user.email}</h1><a href='/auth/logout'>Sign out</a>"


# Protected route: require_auth() responds 401 when there is no session.
@app.get("/api/me")
def me(user: User = Depends(require_auth())):
    return user.model_dump()
