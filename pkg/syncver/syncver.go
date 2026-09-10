package syncver

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/ha1tch/gorepoman/pkg/config"
	"github.com/ha1tch/gorepoman/pkg/report"
	"github.com/ha1tch/gorepoman/pkg/webhelp"
)

var versionRe = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.]+)?$`)

func GetVersion(root string, cfg *config.Config) string {
	b, err := os.ReadFile(filepath.Join(root, cfg.VersionFile))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func SetVersion(newVer, root string, cfg *config.Config) error {
	if !versionRe.MatchString(newVer) {
		return fmt.Errorf("invalid version %q (want X.Y.Z[-suffix])", newVer)
	}
	if err := os.WriteFile(filepath.Join(root, cfg.VersionFile), []byte(newVer+"\n"), 0644); err != nil {
		return err
	}

	for _, t := range cfg.VersionTargets {
		p := filepath.Join(root, t.File)
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		text := string(b)

		// (?m) enables multiline mode, aligning with Python's re.M
		pat, err := regexp.Compile("(?m)" + t.Match)
		if err != nil {
			return err
		}

		m := pat.FindStringSubmatchIndex(text)
		if m == nil || len(m) < 4 { // Needs at least one capture group (0:1 full match, 2:3 group 1)
			return fmt.Errorf("%s: pattern must match with exactly one group", t.File)
		}

		newText := text[:m[2]] + newVer + text[m[3]:]
		if err := os.WriteFile(p, []byte(newText), 0644); err != nil {
			return err
		}
	}
	return nil
}

func CheckDetail(root string, cfg *config.Config) (bool, string) {
	canon := GetVersion(root, cfg)
	if canon == "" {
		return false, fmt.Sprintf("%s missing or empty", cfg.VersionFile)
	}

	for _, t := range cfg.VersionTargets {
		b, err := os.ReadFile(filepath.Join(root, t.File))
		if err != nil {
			return false, err.Error()
		}
		pat, err := regexp.Compile("(?m)" + t.Match)
		if err != nil {
			return false, err.Error()
		}
		m := pat.FindStringSubmatch(string(b))
		got := "<no match>"
		if len(m) > 1 {
			got = m[1]
		}
		if got != canon {
			return false, fmt.Sprintf("%s: %s != %s", t.File, got, canon)
		}
	}
	return true, canon
}

func Check(root string, cfg *config.Config) bool {
	ok, _ := CheckDetail(root, cfg)
	return ok
}

func Bump(part, root string, cfg *config.Config) (string, error) {
	cur := strings.Split(GetVersion(root, cfg), "-")[0]
	parts := strings.Split(cur, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid current version: %s", cur)
	}
	major, _ := strconv.Atoi(parts[0])
	minor, _ := strconv.Atoi(parts[1])
	patch, _ := strconv.Atoi(parts[2])

	switch part {
	case "major":
		major, minor, patch = major+1, 0, 0
	case "minor":
		minor, patch = minor+1, 0
	case "patch":
		patch++
	}

	newVer := fmt.Sprintf("%d.%d.%d", major, minor, patch)
	return newVer, SetVersion(newVer, root, cfg)
}

// Run executes the syncver command
const syncverHelp = `usage: repoman syncver [-h] {show,set,check,bump-patch,bump-minor,bump-major} ...

repoman syncver -- keeps the VERSION file and every configured
version_targets entry (in .repoman.json) in agreement, so a release
never ships with a version string correct in one file and stale in
another.

Subcommands:
  show          Print the current version and whether every target
                 is in sync (the default with no subcommand given).
  set VERSION    Set an explicit version everywhere at once.
  check          Same check as show, but exits non-zero on a
                 mismatch -- for CI use.
  bump-patch     Increment the patch component (x.y.Z) everywhere.
  bump-minor     Increment the minor component (x.Y.0) everywhere.
  bump-major     Increment the major component (X.0.0) everywhere.

Options:
  -h, --help       Show this help message and exit.
  --format FORMAT  For show/check: text (default), json, or html.
                   json is a stable, queryfy-validated data contract
                   for other systems -- no presentation information.
                   html is a self-contained document in gorepoman's
                   own visual style; an agent with shell access to
                   repoman should relay this output directly rather
                   than re-deriving a rendering from json output.

See https://ha1tch.github.io/gorepoman/docs/repoman-070-releases.html
for how this fits into a full release alongside relcore.
`

func Run(args []string) int {
	args = webhelp.NormalizeBriefFirst(args)
	for _, a := range args {
		if a == "-h" || a == "--help" {
			fmt.Print(syncverHelp)
			fmt.Println(webhelp.SuppressionNote)
			webhelp.PrintIfAvailable(os.Stdout, "repoman-070-releases", args)
			return 0
		}
	}
	format, args := report.ExtractFormat(args)
	cmd := "show"
	if len(args) > 0 {
		cmd = args[0]
	}

	root, cfg, err := config.Load("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
		return 1
	}

	switch cmd {
	case "show":
		status := CheckDetailed(root, &cfg)
		switch format {
		case "text":
			ok, detail := CheckDetail(root, &cfg)
			syncS := "ok"
			if !ok {
				syncS = "MISMATCH — " + detail
			}
			fmt.Printf("version: %s  sync: %s\n", status.Version, syncS)
		case "json":
			if err := Validate(status); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return 1
			}
			if err := report.EmitJSON(os.Stdout, "syncver", "syncver-status", SchemaVersion, status); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return 1
			}
		case "html":
			body, err := renderStatusHTML(status)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return 1
			}
			if err := report.EmitHTML(os.Stdout, "syncver status", "syncver", "syncver-status", body); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return 1
			}
		default:
			fmt.Fprintf(os.Stderr, "unknown format %q (want text, json, or html)\n", format)
			return 1
		}
	case "set":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "error: set requires a version argument")
			return 1
		}
		if err := SetVersion(args[1], root, &cfg); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		fmt.Printf("version set to %s\n", args[1])
	case "check":
		status := CheckDetailed(root, &cfg)
		exit := 0
		if !status.InSync {
			exit = 1
		}
		switch format {
		case "text":
			ok, detail := CheckDetail(root, &cfg)
			if ok {
				fmt.Printf("OK: versions in sync (%s)\n", detail)
			} else {
				fmt.Printf("MISMATCH: %s\n", detail)
			}
		case "json":
			if err := Validate(status); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return 1
			}
			if err := report.EmitJSON(os.Stdout, "syncver", "syncver-status", SchemaVersion, status); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return 1
			}
		case "html":
			body, err := renderStatusHTML(status)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return 1
			}
			if err := report.EmitHTML(os.Stdout, "syncver status", "syncver", "syncver-status", body); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return 1
			}
		default:
			fmt.Fprintf(os.Stderr, "unknown format %q (want text, json, or html)\n", format)
			return 1
		}
		return exit
	case "bump-patch", "bump-minor", "bump-major":
		part := strings.Split(cmd, "-")[1]
		newVer, err := Bump(part, root, &cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		fmt.Printf("version set to %s\n", newVer)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", cmd)
		return 1
	}
	return 0
}
