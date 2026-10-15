package control

import "net/http"

type scenarioCard struct {
	Name        string
	Title       string
	Description string
	Applied     bool
	States      []pinView
}

type scenariosPage struct {
	page
	Cards   []scenarioCard
	Example string
	Error   string
}

func (a *API) showScenarios(w http.ResponseWriter, req *http.Request, f *failure) {
	p := a.project.Load()
	sp := scenariosPage{page: a.page(req, "Scenarios")}
	if f != nil {
		sp.Error = failureText(f)
	}
	applied := a.opts.Memory.Applied()
	for _, s := range p.Scenarios {
		card := scenarioCard{Name: s.Name, Title: s.Title, Description: s.Description, Applied: s.Name == applied}
		if card.Title == "" {
			card.Title = s.Name
		}
		for _, st := range s.States {
			pv := pinView{ID: st.Route, State: st.State}
			if r, ok := p.Route(st.Route); ok {
				pv.Method, pv.Path = string(r.Method), r.Path
			}
			card.States = append(card.States, pv)
		}
		sp.Cards = append(sp.Cards, card)
	}
	if len(p.Scenarios) > 0 {
		sp.Example = p.Scenarios[0].Name
	}
	render(w, "scenarios", http.StatusOK, sp)
}

func (a *API) scenariosPage(w http.ResponseWriter, req *http.Request) {
	a.showScenarios(w, req, nil)
}

func (a *API) applyFromUI(w http.ResponseWriter, req *http.Request) {
	_, f := a.applyScenario(req.PathValue("name"))
	a.showScenarios(w, req, f)
}
