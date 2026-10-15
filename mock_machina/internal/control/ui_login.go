package control

import (
	"bytes"
	"crypto/subtle"
	"html/template"
	"net/http"
	"strings"
)

const sessionCookie = "mockmachina_token"

var lockedTemplate = template.Must(template.ParseFS(uiFiles, "ui/login.html"))

func public(r *http.Request) bool {
	switch {
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, uiPrefix+"static/"):
		return true
	case r.Method == http.MethodPost && r.URL.Path == uiPrefix+"login":
		return true
	}
	return false
}

func (a *API) sessionCookie(r *http.Request) bool {
	c, err := r.Cookie(sessionCookie)
	return err == nil && a.opts.Token != "" && subtle.ConstantTimeCompare([]byte(c.Value), []byte(a.opts.Token)) == 1
}

type lockedView struct {
	Base    string
	Message string
	Form    bool
	Problem string
}

func lockedPage(w http.ResponseWriter, status int, msg, problem string) {
	h := w.Header()
	h.Set("Content-Security-Policy", pageCSP)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "same-origin")
	v := lockedView{Base: uiPrefix, Message: msg, Form: status == http.StatusUnauthorized, Problem: problem}
	if v.Form {
		v.Message = "This mock is open to other machines with a token. start printed it when it began, or it's in MOCKMACHINA_CONTROL_TOKEN."
	}
	var b bytes.Buffer
	if err := lockedTemplate.Execute(&b, v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = b.WriteTo(w)
}

func (a *API) login(w http.ResponseWriter, req *http.Request) {
	token := req.PostFormValue("token")
	if a.opts.Token == "" {
		lockedPage(w, http.StatusForbidden, "the control API only answers this machine; set MOCKMACHINA_CONTROL_TOKEN to allow others", "")
		return
	}
	if subtle.ConstantTimeCompare([]byte(token), []byte(a.opts.Token)) != 1 {
		lockedPage(w, http.StatusUnauthorized, "", "That token doesn't match. Copy it again from start's output.")
		return
	}
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // a mock on a LAN usually serves plain http, where a Secure cookie would never come back; Secure is set over TLS
		Name: sessionCookie, Value: a.opts.Token, Path: uiPrefix,
		HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: req.TLS != nil,
	})
	http.Redirect(w, req, uiPrefix, http.StatusSeeOther)
}
