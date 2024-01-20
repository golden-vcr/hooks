package userauth

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/golden-vcr/hooks"
	"github.com/gorilla/mux"
)

type Server struct {
	origin                string
	twitchClientId        string
	requiredSubscriptions hooks.RequiredSubscriptions
	csrf                  *csrfBuffer
}

func NewServer(origin, twitchClientId string) *Server {
	return &Server{
		origin:                origin,
		twitchClientId:        twitchClientId,
		requiredSubscriptions: hooks.Subscriptions,
		csrf: &csrfBuffer{
			tokens: make([]csrfToken, 0, 8),
		},
	}
}

func (s *Server) RegisterRoutes(r *mux.Router) {
	r.Path("/userauth/start").Methods("GET").HandlerFunc(s.handleStartAuth)
	r.Path("/userauth/finish").Methods("GET").HandlerFunc(s.handleFinishAuth)
}

func (s *Server) handleStartAuth(res http.ResponseWriter, req *http.Request) {
	u, err := url.Parse("https://id.twitch.tv/oauth2/authorize")
	if err != nil {
		panic(err)
	}
	q := u.Query()
	q.Add("response_type", "code")
	q.Add("client_id", s.twitchClientId)
	q.Add("redirect_uri", s.origin+"/userauth/finish")
	q.Add("scope", strings.Join(s.requiredSubscriptions.GetRequiredUserScopes(), " "))
	q.Add("state", s.csrf.generate())
	u.RawQuery = q.Encode()

	res.Header().Set("location", u.String())
	res.WriteHeader(http.StatusSeeOther)
}

func (s *Server) handleFinishAuth(res http.ResponseWriter, req *http.Request) {
	// Verify the CSRF token carried in the 'state' parameter
	tokenValue := req.URL.Query().Get("state")
	if tokenValue == "" {
		http.Error(res, "'state' value not found in URL query params", http.StatusBadRequest)
		return
	}
	if !s.csrf.check(tokenValue) {
		http.Error(res, "CSRF token verification failed", http.StatusBadRequest)
		return
	}

	// Verify that all requested scopes were granted
	scopeValue := req.URL.Query().Get("scope")
	if scopeValue == "" {
		http.Error(res, "'scope' value not found in URL query params", http.StatusBadRequest)
		return
	}
	scopes := strings.Split(scopeValue, " ")
	if len(scopes) == 0 {
		http.Error(res, "'scope' must specify at least one user scope", http.StatusBadRequest)
		return
	}
	for _, desiredScope := range s.requiredSubscriptions.GetRequiredUserScopes() {
		wasGranted := false
		for _, scope := range scopes {
			if scope == desiredScope {
				wasGranted = true
				break
			}
		}
		if !wasGranted {
			http.Error(res, fmt.Sprintf("required scope '%s' was not granted", desiredScope), http.StatusBadRequest)
			return
		}
	}

	res.Header().Set("Content-Type", "text/html; charset=utf-8")
	res.Write([]byte("<!DOCTYPE html><html><head><title>OK</title></head><body><h1>Success!</h1><p>Access granted. You may close this window.</p></body></html>"))
}
