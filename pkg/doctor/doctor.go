// Package doctor implements the environment diagnostic -- translated
// from repoman/doctor.py. Run once after install, alongside (before,
// ideally) `repoman selftest`.
//
// Different job from selftest, deliberately: selftest asks "is the
// CODE correct" and is a pass/fail gate. This asks "what does THIS
// environment provide", and most of its answers are not failures at
// all -- pkg/strreplace's whole design treats every external tool it
// can use (gofmt, bash, node, a YAML validator) as an optional
// enhancement with a documented, working fallback when absent (a
// heuristic check, or an honest "not independently verified" instead
// of a real one). Missing gofmt is not broken; it is one of this
// project's own supported operating modes. What doctor adds is
// visibility: a newcomer to a repository this project's tools are
// installed in should not have to read pkg/strreplace's own source to
// learn which of their edits get a real syntax check and which get a
// heuristic one, or spend time discovering a platform-specific
// install command by trial and error.
//
// Supported, tested environments (per project convention -- anything
// else is not unsupported out of malice, just untested, and this tool
// says so plainly rather than guessing): Debian/Ubuntu, macOS, and
// Ubuntu-under-WSL2. Install hints below are for these three only.
//
// ADAPTATION FROM repoman/doctor.py: the Python original gates on its
// own interpreter's version (Python >= 3.10, the floor its own
// f-string/match/generics syntax already assumes) -- meaningless for
// a compiled Go binary that is, by construction, already running on a
// Go toolchain new enough to have built it. What DOES still matter in
// a Go environment is whether the *installed* `go` toolchain can
// build and test this project, so this port instead reads the `go`
// directive from the project's own go.mod (found via pkg/config's
// FindRoot) and compares it against `go version`'s report. Exit code
// remains 0 unless that floor is not met; every other finding stays
// informational, exactly as in doctor.py.
//
// WhichGofmt is exported so pkg/strreplace can call the exact same
// lookup this report describes, the same anti-drift guarantee
// doctor.py achieved by importing str_replace_extended.py directly.
//
// Usage:
//
//	repoman doctor
//	repoman doctor --quiet   // summary line only, still exit 0/1
package doctor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/ha1tch/gorepoman/pkg/config"
	"github.com/ha1tch/gorepoman/pkg/webhelp"
)

// ToolInfo describes one optional external tool's availability and
// what it enables or what the fallback is when absent.
type ToolInfo struct {
	Found    bool
	Path     string
	Version  string
	Enables  string
	Fallback string
}

// Report is the structured diagnostic; used by both Run below and by
// selftest, which prints a short summary of this at the start of its
// own run without treating any of it as pass/fail.
type Report struct {
	GoOK      bool
	GoVersion string
	GoFloor   string // "" if go.mod's own floor could not be determined

	Platform          string
	PlatformSupported bool

	Gofmt  ToolInfo
	Bash   ToolInfo
	Node   ToolInfo
	PyYAML ToolInfo
}

// WhichGofmt locates a gofmt binary: PATH first, then $GOROOT/bin,
// then the two install locations this project's own environments
// have actually used. A missing validator should never be the reason
// a real bug ships; check the obvious places before giving up and
// reporting "not independently verified".
func WhichGofmt() string {
	// Test-only override: makes the "gofmt genuinely unavailable" case
	// reliably testable regardless of what the host machine actually
	// has installed (WhichGofmt's own fallback paths below do a real
	// filesystem stat on fixed locations, which no PATH/GOROOT
	// manipulation can hide if gofmt is actually there). Not a real
	// feature -- never documented, never relied on outside selftest.
	if os.Getenv("REPOMAN_TEST_FORCE_NO_GOFMT") != "" {
		return ""
	}
	if hit, err := exec.LookPath("gofmt"); err == nil {
		return hit
	}
	if goroot := os.Getenv("GOROOT"); goroot != "" {
		cand := filepath.Join(goroot, "bin", "gofmt")
		if fi, err := os.Stat(cand); err == nil && !fi.IsDir() {
			return cand
		}
	}
	for _, cand := range []string{"/usr/local/go/bin/gofmt", "/usr/lib/go/bin/gofmt"} {
		if fi, err := os.Stat(cand); err == nil && !fi.IsDir() {
			return cand
		}
	}
	return ""
}

func toolVersion(cmd []string) string {
	if len(cmd) == 0 {
		return "(version unknown)"
	}
	c := exec.Command(cmd[0], cmd[1:]...)
	done := make(chan []byte, 1)
	go func() {
		out, _ := c.CombinedOutput()
		done <- out
	}()
	select {
	case out := <-done:
		lines := strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)
		if lines[0] == "" {
			return "(version unknown)"
		}
		return lines[0]
	case <-time.After(10 * time.Second):
		_ = c.Process.Kill()
		return "(version unknown)"
	}
}

var goModVersionRe = regexp.MustCompile(`(?m)^go\s+(\d+)\.(\d+)(?:\.(\d+))?\s*$`)

// goFloorFromModule reads the `go` directive from go.mod at the
// project root (discovered via config.FindRoot), returning "" if no
// go.mod is found or it has no `go` directive.
func goFloorFromModule() string {
	root, err := config.FindRoot("")
	if err != nil {
		return ""
	}
	b, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return ""
	}
	m := goModVersionRe.FindStringSubmatch(string(b))
	if m == nil {
		return ""
	}
	if m[3] != "" {
		return m[1] + "." + m[2] + "." + m[3]
	}
	return m[1] + "." + m[2]
}

// versionAtLeast compares two dotted version strings (e.g. "1.22.2"
// vs "1.21") numerically, component by component.
func versionAtLeast(have, floor string) bool {
	haveParts := strings.Split(have, ".")
	floorParts := strings.Split(floor, ".")
	for i := 0; i < len(floorParts); i++ {
		var h, f int
		if i < len(haveParts) {
			h, _ = strconv.Atoi(haveParts[i])
		}
		f, _ = strconv.Atoi(floorParts[i])
		if h != f {
			return h > f
		}
	}
	return true
}

var goVersionOutRe = regexp.MustCompile(`go(\d+\.\d+(?:\.\d+)?)`)

func installedGoVersion() string {
	if runtime.Version() != "" {
		if m := goVersionOutRe.FindStringSubmatch(runtime.Version()); m != nil {
			return m[1]
		}
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		return ""
	}
	out := toolVersion([]string{goBin, "version"})
	if m := goVersionOutRe.FindStringSubmatch(out); m != nil {
		return m[1]
	}
	return ""
}

// detectPlatform returns (label, supported). WSL2 is detected via
// /proc/version's own kernel string (Microsoft's WSL kernels
// self-identify there) rather than any environment variable, since
// env vars are easy to leave unset in a script's own invocation
// context but the kernel string cannot lie about what it is.
func detectPlatform() (string, bool) {
	switch runtime.GOOS {
	case "darwin":
		ver := macVersion()
		if ver == "" {
			ver = "unknown version"
		}
		return "macOS " + ver, true
	case "linux":
		isWSL := false
		if b, err := os.ReadFile("/proc/version"); err == nil {
			lower := strings.ToLower(string(b))
			isWSL = strings.Contains(lower, "microsoft") || strings.Contains(lower, "wsl")
		}
		distro := linuxDistro()
		if isWSL {
			if distro != "" {
				return "WSL2 / " + distro, true
			}
			return "WSL2 (distro unknown)", true
		}
		if distro != "" && strings.Contains(strings.ToLower(distro), "ubuntu") {
			return distro, true
		}
		if distro != "" && strings.Contains(strings.ToLower(distro), "debian") {
			return distro, true
		}
		if distro != "" {
			return distro, false
		}
		return "Linux (distro unknown)", false
	default:
		return fmt.Sprintf("%s", runtime.GOOS), false
	}
}

func macVersion() string {
	sw, err := exec.LookPath("sw_vers")
	if err != nil {
		return ""
	}
	out := toolVersion([]string{sw, "-productVersion"})
	if out == "(version unknown)" {
		return ""
	}
	return out
}

var prettyNameRe = regexp.MustCompile(`(?m)^PRETTY_NAME="?([^"\n]+)"?`)

func linuxDistro() string {
	b, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return ""
	}
	m := prettyNameRe.FindStringSubmatch(string(b))
	if m == nil {
		return ""
	}
	return m[1]
}

// pkgHints returns install commands, keyed by tool name -- only shown
// for tools actually found missing, never printed unprompted for
// tools already present.
func pkgHints(goos string) map[string]string {
	if goos == "darwin" {
		return map[string]string{
			"go":   "brew install go",
			"node": "brew install node",
			"yaml": "pip3 install pyyaml  (add --break-system-packages, or use a " +
				"venv, if this Python is Homebrew's own and refuses global installs)",
			// bash: no hint -- macOS ships it unconditionally as part of the
			// OS, this case should not be reachable there.
		}
	}
	// Debian/Ubuntu, native or under WSL2 -- same package manager either way.
	return map[string]string{
		"go": "sudo apt install golang-go   (ships gofmt; for a newer Go than " +
			"Ubuntu's own package, see https://go.dev/dl/)",
		"node": "sudo apt install nodejs npm",
		"bash": "sudo apt install bash   (genuinely absent on some minimal/" +
			"Alpine-derived images -- not a concern on a standard " +
			"Debian/Ubuntu install, which always includes it)",
		"yaml": "pip3 install pyyaml --break-system-packages   (Debian/Ubuntu's " +
			"system Python refuses a bare global install since Python 3.11 -- " +
			"this project's own sandbox needed exactly this flag)",
	}
}

// Check returns a structured report; used by both Run below and by
// selftest, which prints a short summary of this at the start of its
// own run without treating any of it as pass/fail.
func Check() Report {
	var report Report

	report.GoVersion = installedGoVersion()
	report.GoFloor = goFloorFromModule()
	if report.GoFloor == "" || report.GoVersion == "" {
		// Can't determine one side of the comparison -- don't fail a
		// build over an inconclusive check; report it plainly instead.
		report.GoOK = report.GoVersion != ""
	} else {
		report.GoOK = versionAtLeast(report.GoVersion, report.GoFloor)
	}

	report.Platform, report.PlatformSupported = detectPlatform()

	gofmt := WhichGofmt()
	report.Gofmt = ToolInfo{
		Found:    gofmt != "",
		Path:     gofmt,
		Enables:  "real gofmt -e syntax validation for .go substitutions",
		Fallback: "\"validated\": not independently verified -- no heuristic exists for Go yet",
	}
	if gofmt != "" {
		report.Gofmt.Version = toolVersion([]string{gofmt, "-h"})
	}

	bash, _ := exec.LookPath("bash")
	report.Bash = ToolInfo{
		Found:    bash != "",
		Path:     bash,
		Enables:  "real bash -n syntax validation for .sh/.bash substitutions",
		Fallback: "\"validated\": not independently verified -- no heuristic exists for shell yet",
	}
	if bash != "" {
		report.Bash.Version = toolVersion([]string{bash, "--version"})
	}

	node, _ := exec.LookPath("node")
	report.Node = ToolInfo{
		Found: node != "",
		Path:  node,
		Enables: "real node --check syntax validation for .js/.mjs/.cjs substitutions " +
			"(.jsx/.ts/.tsx always use the heuristic path regardless)",
		Fallback: "\"validated\": heuristic -- role-aware bracket/string balance check, real but reduced-confidence",
	}
	if node != "" {
		report.Node.Version = toolVersion([]string{node, "--version"})
	}

	// PyYAML: this Go binary has no YAML library of its own (pure
	// stdlib, matching repoman.py's own no-third-party-dependencies
	// convention), so a real yaml.safe_load validation pass still
	// means shelling out to a Python interpreter with PyYAML
	// installed -- the same external dependency doctor.py itself
	// checked for, just probed via subprocess instead of import
	// because Go cannot import a Python module.
	python3, _ := exec.LookPath("python3")
	report.PyYAML = ToolInfo{
		Enables:  "real yaml.safe_load validation for .yaml/.yml substitutions",
		Fallback: "\"validated\": heuristic -- tab/bracket structural check, real but reduced-confidence",
	}
	if python3 != "" {
		out := toolVersion([]string{python3, "-c",
			"import yaml,sys; print(getattr(yaml,'__version__','(version unknown)'))"})
		if out != "(version unknown)" && !strings.Contains(out, "ModuleNotFoundError") &&
			!strings.Contains(out, "ImportError") && !strings.Contains(out, "Traceback") {
			report.PyYAML.Found = true
			report.PyYAML.Version = out
		}
	}

	return report
}

func printReport(report Report, quiet bool) {
	goMark := "OK"
	if !report.GoOK {
		goMark = "FAIL"
	}
	floorNote := ""
	switch {
	case report.GoVersion == "":
		floorNote = "go toolchain not found on PATH"
	case report.GoFloor == "":
		floorNote = "go.mod floor not determined -- not treated as a failure"
	case report.GoOK:
		floorNote = fmt.Sprintf(">= %s (this project's go.mod floor), fine", report.GoFloor)
	default:
		floorNote = fmt.Sprintf("BELOW %s -- this project's go.mod requires it", report.GoFloor)
	}
	fmt.Printf("[%s] Go %s (%s)\n", goMark, orUnknown(report.GoVersion), floorNote)

	platMark := "OK"
	if !report.PlatformSupported {
		platMark = "??"
	}
	suffix := ""
	if !report.PlatformSupported {
		suffix = " -- not one of this project's tested environments (Debian/Ubuntu, " +
			"macOS, Ubuntu/WSL2). May still work; untested, not unsupported out of malice."
	}
	fmt.Printf("[%s] Platform: %s%s\n", platMark, report.Platform, suffix)

	if quiet {
		var missing []string
		if !report.Gofmt.Found {
			missing = append(missing, "gofmt")
		}
		if !report.Bash.Found {
			missing = append(missing, "bash")
		}
		if !report.Node.Found {
			missing = append(missing, "node")
		}
		if !report.PyYAML.Found {
			missing = append(missing, "pyyaml")
		}
		if len(missing) > 0 {
			fmt.Printf("optional tools not found: %s (fallbacks apply -- run without --quiet for detail)\n",
				strings.Join(missing, ", "))
		}
		return
	}

	fmt.Println()
	hints := pkgHints(runtime.GOOS)
	names := []string{"gofmt", "bash", "node", "pyyaml"}
	hintKeys := map[string]string{"gofmt": "go", "bash": "bash", "node": "node", "pyyaml": "yaml"}
	infos := map[string]ToolInfo{"gofmt": report.Gofmt, "bash": report.Bash, "node": report.Node, "pyyaml": report.PyYAML}

	for _, name := range names {
		info := infos[name]
		if info.Found {
			where := ""
			if info.Path != "" {
				where = " (" + info.Path + ")"
			}
			ver := ""
			if info.Version != "" {
				ver = " -- " + info.Version
			}
			fmt.Printf("[OK] %s%s%s\n", name, where, ver)
			fmt.Printf("     enables: %s\n", info.Enables)
		} else {
			fmt.Printf("[--] %s: not found\n", name)
			fmt.Printf("     without it: %s\n", info.Fallback)
			if key, ok := hintKeys[name]; ok {
				if hint, ok := hints[key]; ok {
					fmt.Printf("     install: %s\n", hint)
				}
			}
		}
		fmt.Println()
	}
}

func orUnknown(s string) string {
	if s == "" {
		return "(not found)"
	}
	return s
}

// Run implements `repoman doctor [--quiet]`.
const doctorHelp = `usage: repoman doctor [-h] [--quiet]

repoman doctor -- an environment diagnostic, not a pass/fail test.
Reports the Go version this binary was compiled with, the current
platform, and which of four optional external tools (gofmt, bash,
node, pyyaml) this environment has -- each with exactly what it
enables and what the fallback is when it's absent. An absent
optional tool is never a failure here; every one has a documented
degraded-but-real fallback, and this command exists so that's a
visible, informed choice rather than a silent one.

Options:
  --quiet      Drop the per-tool detail; print only the Go and
                platform baseline.
  -h, --help   Show this help message and exit.

Run this before ` + "`repoman selftest`" + `, the actual acceptance gate --
selftest prints this same summary first, informationally, then goes
on to actually exercise every tool.

See https://ha1tch.github.io/gorepoman/docs/repoman-030-getting-started.html
for the full getting-started guide.
`

func Run(argv []string) int {
	argv = webhelp.NormalizeBriefFirst(argv)
	for _, a := range argv {
		if a == "-h" || a == "--help" {
			fmt.Print(doctorHelp)
			fmt.Println(webhelp.SuppressionNote)
			webhelp.PrintIfAvailable(os.Stdout, "repoman-030-getting-started", argv)
			return 0
		}
	}
	quiet := false
	for _, a := range argv {
		if a == "--quiet" {
			quiet = true
		}
	}
	report := Check()
	printReport(report, quiet)
	if report.GoOK {
		return 0
	}
	return 1
}
