// Package board aggregates several local project checkouts into one
// cross-project view. It does this by invoking repoman's own already-
// existing `register list --format json` and `waveprogress --show
// --format json` against each project directory and merging their
// envelopes -- not by re-parsing TRACKING.md or WAVE_TRACKING.md
// itself. The five reporting commands already own that parsing and
// its schema; board's only job is enumerate, fetch, merge, present.
//
// Scope, deliberately: local directories only, given as positional
// arguments. No manifest file, no network fetch, no workspace-issue
// merging -- join/leave/the workspace participants file don't exist
// yet, so there is nothing real to merge from there. This covers what
// can actually be built and verified today; the rest is a later,
// separate pass.
package board

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ha1tch/gorepoman/pkg/register"
	"github.com/ha1tch/gorepoman/pkg/report"
	"github.com/ha1tch/gorepoman/pkg/waveprogress"
	"github.com/ha1tch/gorepoman/pkg/webhelp"
)

// ProjectReport is one project's rolled-up status. RegisterError and
// WaveError are "" on success -- a project with no wave tracking
// configured at all is not a failure of board itself, so it's
// recorded as a plain, readable message rather than making the whole
// run fail.
type ProjectReport struct {
	Name           string                    `json:"name"`
	Path           string                    `json:"path"`
	RegisterError  string                    `json:"register_error"`
	OpenItems      []register.ItemSummary    `json:"open_items"`
	WaveError      string                    `json:"wave_error"`
	WaveOverallPct int                       `json:"wave_overall_pct"`
	Waves          []waveprogress.WaveStatus `json:"waves"`
}

// BoardReport is the structured shape of `repoman board`.
type BoardReport struct {
	Projects []ProjectReport `json:"projects"`
}

type jsonEnvelope struct {
	Data json.RawMessage `json:"data"`
}

// runIn shells out to repomanPath with args, cwd set to dir. Returns
// stdout on a clean exit; on a non-zero exit, returns stderr's own
// message (that's the real reason -- e.g. "no wave-tracking document
// at ..."), falling back to the raw error only if stderr was empty.
func runIn(repomanPath, dir string, args ...string) ([]byte, error) {
	cmd := exec.Command(repomanPath, args...)
	cmd.Dir = dir
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errOut.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("%s", msg)
	}
	return out.Bytes(), nil
}

func gatherProject(repomanPath, dir string) ProjectReport {
	pr := ProjectReport{
		Name:      filepath.Base(dir),
		Path:      dir,
		OpenItems: []register.ItemSummary{},
		Waves:     []waveprogress.WaveStatus{},
	}

	if out, err := runIn(repomanPath, dir, "register", "list", "--format", "json"); err != nil {
		pr.RegisterError = err.Error()
	} else {
		var env jsonEnvelope
		if err := json.Unmarshal(out, &env); err != nil {
			pr.RegisterError = "malformed register output: " + err.Error()
		} else if err := json.Unmarshal(env.Data, &pr.OpenItems); err != nil {
			pr.RegisterError = "malformed register data: " + err.Error()
		}
	}

	if out, err := runIn(repomanPath, dir, "waveprogress", "--show", "--format", "json"); err != nil {
		pr.WaveError = err.Error()
	} else {
		var env jsonEnvelope
		var data struct {
			Waves      []waveprogress.WaveStatus `json:"waves"`
			OverallPct int                       `json:"overall_pct"`
		}
		if err := json.Unmarshal(out, &env); err != nil {
			pr.WaveError = "malformed waveprogress output: " + err.Error()
		} else if err := json.Unmarshal(env.Data, &data); err != nil {
			pr.WaveError = "malformed waveprogress data: " + err.Error()
		} else {
			pr.Waves = data.Waves
			pr.WaveOverallPct = data.OverallPct
		}
	}

	return pr
}

const boardHelp = `usage: repoman board [-h] [--format text|json|html] [--definition PATH] dir [dir ...]

Aggregate several local project checkouts into one cross-project view,
by invoking this same binary's 'register list' and 'waveprogress
--show' against each directory (each with --format json) and merging
the results. Each directory must have its own .repoman.json; a
project with no wave tracking configured is reported as such, not
treated as an error.

positional arguments:
  dir                   one or more local project directories

options:
  -h, --help            show this help message and exit
  --format FORMAT       text (default), json, or html -- see
                        repoman-055-format.md
  --definition PATH     render a configured, single-project board from
                        a board definition file (schema
                        gorepoman.workspace.board/1) instead of the
                        default multi-project aggregate view -- still
                        requires at least one dir argument. Chooses one
                        of four axes (status, priority, theme,
                        dependency_order) and lays register items out
                        into that axis's columns. See
                        repoman-085-board.md.

Local directories only: no manifest file, no network fetch, no
workspace-issue merging (see repoman-086-workspace.md for that
separate mechanism). See repoman-055-format.md.
`

// Run implements `repoman board <dir> [<dir> ...] [--format ...]`.
func Run(args []string) int {
	args = webhelp.NormalizeBriefFirst(args)
	format, args := report.ExtractFormat(args)
	definitionPath := ""
	var rest []string
	for i := 0; i < len(args); i++ {
		if args[i] == "-h" || args[i] == "--help" {
			fmt.Print(boardHelp)
			fmt.Println(webhelp.SuppressionNote)
			webhelp.PrintIfAvailable(os.Stdout, "repoman-085-board", args)
			return 0
		}
		if args[i] == "--definition" {
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "--definition requires a path")
				return 1
			}
			definitionPath = args[i+1]
			i++
			continue
		}
		rest = append(rest, args[i])
	}
	args = rest
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: repoman board <dir> [<dir> ...] [--format text|json|html] [--definition PATH]")
		return 1
	}

	repomanPath, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	var projects []ProjectReport
	for _, dir := range args {
		abs, err := filepath.Abs(dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", dir, err)
			return 1
		}
		if fi, err := os.Stat(abs); err != nil || !fi.IsDir() {
			fmt.Fprintf(os.Stderr, "%s: not a directory\n", dir)
			return 1
		}
		projects = append(projects, gatherProject(repomanPath, abs))
	}
	if definitionPath != "" {
		return runDefinition(definitionPath, projects, format)
	}

	data := BoardReport{Projects: projects}

	switch format {
	case "text":
		printText(data)
		return 0
	case "json":
		if err := Validate(data); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		return report.EmitOrErr(report.EmitJSON(os.Stdout, "board", "board-report", SchemaVersion, data))
	case "html":
		body, err := renderReportHTML(data)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		return report.EmitOrErr(report.EmitHTML(os.Stdout, "cross-project board", "board", "board-report", body))
	default:
		fmt.Fprintf(os.Stderr, "unknown format %q (want text, json, or html)\n", format)
		return 1
	}
}

func printText(data BoardReport) {
	for i, p := range data.Projects {
		if i > 0 {
			fmt.Println()
		}
		fmt.Printf("%s  (%s)\n", p.Name, p.Path)
		if p.RegisterError != "" {
			fmt.Printf("  register: %s\n", p.RegisterError)
		} else if len(p.OpenItems) == 0 {
			fmt.Println("  register: no open items")
		} else {
			for _, it := range p.OpenItems {
				fmt.Printf("  %-6s %s  %s  [%s]  %s\n", it.ID, it.Status, it.Priority, it.Theme, it.Title)
			}
		}
		if p.WaveError != "" {
			fmt.Printf("  waves: %s\n", p.WaveError)
		} else {
			fmt.Printf("  waves: %d%% overall\n", p.WaveOverallPct)
		}
	}
}
