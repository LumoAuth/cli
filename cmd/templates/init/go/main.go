// LumoAuth + chi starter.
//
// Wires /auth/login, /auth/callback, /auth/logout via lumo-auth-go's
// chi-compatible middleware. Protects /api/me behind a session cookie.
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/joho/godotenv"
	lumoauth "github.com/lumoauth/lumo-auth-go"
)

func main() {
	_ = godotenv.Load()

	auth, err := lumoauth.NewWebAuth(lumoauth.WebConfig{
		BaseURL:       os.Getenv("LUMO_BASE_URL"),
		Organization:  os.Getenv("LUMO_ORG_ID"),
		ClientID:      os.Getenv("LUMO_CLIENT_ID"),
		ClientSecret:  os.Getenv("LUMO_CLIENT_SECRET"),
		SessionSecret: os.Getenv("SESSION_SECRET"),
		CallbackPath:  "/auth/callback",
	})
	if err != nil {
		log.Fatal(err)
	}

	r := chi.NewRouter()
	r.Mount("/auth", auth.Router())

	r.Group(func(r chi.Router) {
		r.Use(auth.Middleware)
		r.Get("/", func(w http.ResponseWriter, req *http.Request) {
			user := lumoauth.UserFromContext(req.Context())
			if user == nil {
				http.Redirect(w, req, "/auth/login", http.StatusFound)
				return
			}
			fmt.Fprintf(w, "<h1>Hi %s</h1><a href=\"/auth/logout\">Sign out</a>", user.Email)
		})
		r.Get("/api/me", func(w http.ResponseWriter, req *http.Request) {
			user := lumoauth.UserFromContext(req.Context())
			if user == nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			fmt.Fprintf(w, "%v", user)
		})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	log.Printf("listening on http://localhost:%s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}
