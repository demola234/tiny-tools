package control

import (
	"bytes"
	"net/http"
	"strconv"
	"strings"
)

var trafficFilters = []struct{ key, label string }{
	{"", "All"}, {"errors", "Errors"}, {"proxied", "Real backend"}, {"webhooks", "Webhooks"},
}

func keep(filter string, e event) bool {
	switch filter {
	case "errors":
		return e.failed()
	case "proxied":
		return e.Proxied
	case "webhooks":
		return e.Webhook
	default:
		return true
	}
}

type trafficRow struct {
	Base     string
	Time     string
	Method   string
	Path     string
	Route    string
	State    string
	Reason   string
	Status   string
	Duration string
	Tags     []string
	Failed   bool
}

func rowFor(e event) trafficRow {
	r := trafficRow{
		Base: uiPrefix, Time: e.Time.Format("15:04:05"), Method: e.Method, Path: e.Path,
		Route: e.Route, State: e.State, Reason: e.Reason, Status: e.GRPCCode, Failed: e.failed(),
		Duration: strconv.FormatInt(e.DurationMS, 10) + " ms",
	}
	if r.Status == "" && e.Status != 0 {
		r.Status = strconv.Itoa(e.Status)
	}
	if e.Rule > 0 {
		r.Reason += " " + strconv.Itoa(e.Rule)
	}
	if e.Proxied {
		r.Tags = append(r.Tags, "real backend")
	}
	if e.Webhook {
		r.Tags = append(r.Tags, "webhook")
	}
	if e.Fault != "" {
		r.Tags = append(r.Tags, "fault: "+e.Fault)
	}
	if e.Problem != "" {
		problem := e.Problem
		if e.Suggestion != "" {
			problem += " (closest: " + e.Suggestion + ")"
		}
		r.Tags = append(r.Tags, problem)
	}
	return r
}

type filterTab struct {
	Href    string
	Label   string
	Current bool
}

type trafficPage struct {
	page
	Filters []filterTab
	Rows    []trafficRow
}

func (a *API) trafficView(w http.ResponseWriter, req *http.Request) {
	filter := req.FormValue("filter")
	tp := trafficPage{page: a.page(req, "Traffic")}
	if filter != "" {
		tp.Events += "?filter=" + filter
	}
	for _, f := range trafficFilters {
		href := uiPrefix + "traffic"
		if f.key != "" {
			href += "?filter=" + f.key
		}
		tp.Filters = append(tp.Filters, filterTab{Href: href, Label: f.label, Current: f.key == filter})
	}
	for _, e := range a.events.recent() {
		if keep(filter, e) {
			tp.Rows = append(tp.Rows, rowFor(e))
		}
	}
	render(w, "traffic", http.StatusOK, tp)
}

func (a *API) uiEvents(w http.ResponseWriter, req *http.Request) {
	filter := req.FormValue("filter")
	a.serveEvents(w, req, func(m message) (string, bool) {
		e, ok := m.data.(event)
		if !ok {
			return "{}", true
		}
		if !keep(filter, e) {
			return "", false
		}
		var b bytes.Buffer
		if err := pages["traffic"].ExecuteTemplate(&b, "row", rowFor(e)); err != nil {
			return "", false
		}
		return strings.ReplaceAll(b.String(), "\n", " "), true
	})
}
