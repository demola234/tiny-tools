package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/demola234/tiny-tools/mock_machina/internal/config"
	"github.com/demola234/tiny-tools/mock_machina/internal/model"
)

func newStateListCmd() *cobra.Command {
	var dir string
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "list [ROUTE]",
		Short: "Show routes and their states, with the active one marked",
		Args:  usageArgs(cobra.MaximumNArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := projectDir(cmd, dir)
			if err != nil {
				return err
			}
			proj, err := loadProject(dir, cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if len(args) == 0 {
				return listRoutes(out, proj.Routes, asJSON)
			}
			r, err := config.FindRoute(proj, args[0])
			if err != nil {
				return err
			}
			return listStates(out, r, asJSON)
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "", dirFlagUsage)
	cmd.Flags().BoolVar(&asJSON, "json", false, "print JSON for scripts and tools")
	return cmd
}

func loadProject(dir string, stderr io.Writer) (*model.Project, error) {
	proj, probs, err := config.Load(dir)
	if err != nil {
		return nil, fmt.Errorf("can't read project folder %s: %w", dir, err)
	}
	if probs.HasErrors() {
		fix := "fix them first"
		if len(probs) == 1 {
			fix = "fix it first"
		}
		_, _ = fmt.Fprintf(stderr, "%s\n%s; %s\n", probs, probs.Summary(), fix)
		return nil, &ExitError{Code: ExitFailure}
	}
	return proj, nil
}

type routeSummary struct {
	Route  string   `json:"route"`
	Method string   `json:"method"`
	Path   string   `json:"path"`
	Active string   `json:"active"`
	States []string `json:"states"`
}

type routeDetail struct {
	Route  string        `json:"route"`
	Method string        `json:"method"`
	Path   string        `json:"path"`
	Active string        `json:"active"`
	States []stateDetail `json:"states"`
}

type stateDetail struct {
	Name   string `json:"name"`
	Status int    `json:"status"`
}

func listRoutes(out io.Writer, routes []*model.Route, asJSON bool) error {
	if asJSON {
		summaries := make([]routeSummary, len(routes))
		for i, r := range routes {
			summaries[i] = routeSummary{r.ID, string(r.Method), r.Path, r.Active, r.States.Names()}
		}
		return json.NewEncoder(out).Encode(summaries)
	}
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ROUTE\tMETHOD\tPATH\tACTIVE")
	for _, r := range routes {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", r.ID, r.Method, r.Path, r.Active)
	}
	return tw.Flush()
}

func listStates(out io.Writer, r *model.Route, asJSON bool) error {
	if asJSON {
		states := make([]stateDetail, len(r.States))
		for i, ns := range r.States {
			states[i] = stateDetail{ns.Name, ns.State.EffectiveStatus()}
		}
		return json.NewEncoder(out).Encode(routeDetail{r.ID, string(r.Method), r.Path, r.Active, states})
	}
	_, _ = fmt.Fprintf(out, "%s  %s %s\n", r.ID, r.Method, r.Path)
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	for _, ns := range r.States {
		marker := " "
		if ns.Name == r.Active {
			marker = "*"
		}
		_, _ = fmt.Fprintf(tw, "%s %s\t%d\n", marker, ns.Name, ns.State.EffectiveStatus())
	}
	return tw.Flush()
}
