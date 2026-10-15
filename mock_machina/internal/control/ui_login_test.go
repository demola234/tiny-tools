package control_test

import (
	"strings"
	"testing"
)

const elsewhere = "192.168.1.20:50000"

func TestUILogin(t *testing.T) {
	t.Parallel()

	f := newFixture(t, "s3cret")
	away := map[string]string{"RemoteAddr": elsewhere}
	form := map[string]string{"RemoteAddr": elsewhere, "Content-Type": "application/x-www-form-urlencoded"}

	rec := f.raw(t, "GET", "/__mockmachina/ui/", "", away)
	body := rec.Body.String()
	if rec.Code != 401 || !strings.Contains(body, `<form method="post" action="/__mockmachina/ui/login"`) || strings.Contains(body, "users") {
		t.Errorf("locked page = %d, want a login form that names no routes:\n%s", rec.Code, body)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("locked page Content-Type %q", ct)
	}
	if got := rec.Header().Get("Referrer-Policy"); got != "same-origin" {
		t.Errorf("Referrer-Policy %q: browsers send Origin: null with no-referrer, and the origin check refuses that", got)
	}

	rec = f.raw(t, "POST", "/__mockmachina/ui/login", "token=nope", form)
	if rec.Code != 401 || rec.Header().Get("Set-Cookie") != "" || !strings.Contains(rec.Body.String(), "doesn&#39;t match") {
		t.Errorf("wrong token = %d, cookie %q", rec.Code, rec.Header().Get("Set-Cookie"))
	}

	rec = f.raw(t, "POST", "/__mockmachina/ui/login", "token=s3cret", form)
	cookie := rec.Header().Get("Set-Cookie")
	if rec.Code != 303 || rec.Header().Get("Location") != "/__mockmachina/ui/" {
		t.Fatalf("right token = %d, Location %q", rec.Code, rec.Header().Get("Location"))
	}
	for _, part := range []string{"mockmachina_token=s3cret", "Path=/__mockmachina/ui/", "HttpOnly", "SameSite=Strict"} {
		if !strings.Contains(cookie, part) {
			t.Errorf("cookie %q has no %s", cookie, part)
		}
	}
}

func TestUILogin_Cookie(t *testing.T) {
	t.Parallel()

	f := newFixture(t, "s3cret")
	away := map[string]string{"RemoteAddr": elsewhere}
	withCookie := map[string]string{"RemoteAddr": elsewhere, "Cookie": "mockmachina_token=s3cret"}
	if rec := f.raw(t, "GET", "/__mockmachina/ui/", "", withCookie); rec.Code != 200 {
		t.Errorf("dashboard with the cookie = %d", rec.Code)
	}
	if code, _ := f.call(t, "GET", "/__mockmachina/routes", "", withCookie); code != 401 {
		t.Errorf("JSON API with only the cookie = %d, want 401", code)
	}
	wrong := map[string]string{"RemoteAddr": elsewhere, "Cookie": "mockmachina_token=nope"}
	if rec := f.raw(t, "GET", "/__mockmachina/ui/", "", wrong); rec.Code != 401 {
		t.Errorf("dashboard with a wrong cookie = %d", rec.Code)
	}
	if rec := f.raw(t, "GET", "/__mockmachina/ui/static/style.css", "", away); rec.Code != 200 {
		t.Errorf("static file from another machine = %d", rec.Code)
	}
	evil := map[string]string{"RemoteAddr": elsewhere, "Content-Type": "application/x-www-form-urlencoded", "Origin": "https://evil.example"}
	if rec := f.raw(t, "POST", "/__mockmachina/ui/login", "token=s3cret", evil); rec.Code != 403 {
		t.Errorf("login from another site = %d, want 403", rec.Code)
	}
}

func TestUILogin_NoToken(t *testing.T) {
	t.Parallel()

	rec := newFixture(t, "").raw(t, "GET", "/__mockmachina/ui/", "", map[string]string{"RemoteAddr": elsewhere})
	body := rec.Body.String()
	if rec.Code != 403 || rec.Header().Get("Content-Type") != "text/html; charset=utf-8" || strings.Contains(body, "<form") || !strings.Contains(body, "MOCKMACHINA_CONTROL_TOKEN") {
		t.Errorf("no token configured = %d:\n%s", rec.Code, body)
	}
}
