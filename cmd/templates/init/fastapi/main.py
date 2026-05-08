"""LumoAuth + FastAPI starter.

Run:  uvicorn main:app --reload
Visit http://localhost:8000 — you'll be redirected to LumoAuth to sign in.
"""
import os

from dotenv import load_dotenv
from fastapi import FastAPI, Depends, Request
from fastapi.responses import HTMLResponse, RedirectResponse
from starlette.middleware.sessions import SessionMiddleware

from lumoauth.fastapi import lumo_auth_router, get_current_user, User

load_dotenv()

app = FastAPI()

app.add_middleware(SessionMiddleware, secret_key=os.environ["SESSION_SECRET"])

# Mounts /auth/login, /auth/callback, /auth/logout under the prefix.
app.include_router(
    lumo_auth_router(
        base_url=os.environ["LUMO_BASE_URL"],
        organization=os.environ["LUMO_ORG_ID"],
        client_id=os.environ["LUMO_CLIENT_ID"],
        client_secret=os.environ["LUMO_CLIENT_SECRET"],
        callback_path="/auth/callback",
    ),
    prefix="/auth",
)


@app.get("/", response_class=HTMLResponse)
def root(request: Request, user: User | None = Depends(get_current_user)):
    if user is None:
        return RedirectResponse("/auth/login")
    return f"<h1>Hi {user.email}</h1><a href='/auth/logout'>Sign out</a>"


@app.get("/api/me")
def me(user: User = Depends(get_current_user)):
    if user is None:
        return {"error": "unauthorized"}, 401
    return user.model_dump()
