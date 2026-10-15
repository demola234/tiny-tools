package control

import "net/http"

type coverageChip struct {
	Name   string
	Served bool
}

type coverageRow struct {
	ID     string
	Method string
	Path   string
	Served int
	Total  int
	Chips  []coverageChip
}

type coveragePage struct {
	page
	Summary coverageView
	Gaps    bool
	Filters []filterTab
	Rows    []coverageRow
}

func (a *API) coveragePage(w http.ResponseWriter, req *http.Request) {
	gaps := req.FormValue("filter") == "gaps"
	cp := coveragePage{page: a.page(req, "Coverage"), Summary: a.coverageView(), Gaps: gaps}
	cp.Filters = []filterTab{
		{Href: uiPrefix + "coverage", Label: "All routes", Current: !gaps},
		{Href: uiPrefix + "coverage?filter=gaps", Label: "Not fully served", Current: gaps},
	}
	for _, r := range a.project.Load().Routes {
		row := coverageRow{ID: r.ID, Method: string(r.Method), Path: r.Path, Total: len(r.States)}
		if r.Operation != "" {
			row.Path = r.Operation
		}
		for _, s := range r.States {
			served := a.opts.Memory.Served(r.ID, s.Name)
			if served {
				row.Served++
			}
			row.Chips = append(row.Chips, coverageChip{Name: s.Name, Served: served})
		}
		if gaps && row.Served == row.Total {
			continue
		}
		cp.Rows = append(cp.Rows, row)
	}
	render(w, "coverage", http.StatusOK, cp)
}
