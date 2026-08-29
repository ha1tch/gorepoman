package main

import (
	"fmt"
	"os"

	"github.com/ha1tch/gorepoman/pkg/addwave"
	"github.com/ha1tch/gorepoman/pkg/badcode"
	"github.com/ha1tch/gorepoman/pkg/doctor"
	"github.com/ha1tch/gorepoman/pkg/ed"
	"github.com/ha1tch/gorepoman/pkg/gomod"
	"github.com/ha1tch/gorepoman/pkg/guards"
	"github.com/ha1tch/gorepoman/pkg/register"
	"github.com/ha1tch/gorepoman/pkg/relcore"
	"github.com/ha1tch/gorepoman/pkg/roles"
	"github.com/ha1tch/gorepoman/pkg/selftest"
	"github.com/ha1tch/gorepoman/pkg/strreplace"
	"github.com/ha1tch/gorepoman/pkg/syncver"
	"github.com/ha1tch/gorepoman/pkg/waveprogress"
)

// version is overridden at build time via
// -ldflags "-X main.version=...". "dev" means a local, non-release
// build -- never trust it as a real version identifier.
var version = "dev"

func printUsage() {
	fmt.Println("repoman — repository-discipline tooling (Go translation)")
	fmt.Println("Usage: repoman <command> [args...]")
	fmt.Println("\nCommands:")
	fmt.Println("  version      Print the build version")
	fmt.Println("  badcode      Scan for forbidden strings (local config, never in a repo)")
	fmt.Println("  doctor       Environment diagnostic")
	fmt.Println("  ed           Journaled text editing")
	fmt.Println("  roles        Syntactic-role auditor")
	fmt.Println("  strreplace   Format-aware base64 substitution")
	fmt.Println("  register     Live-register operations (TRACKING.md)")
	fmt.Println("  guards       Dormant-guard registry")
	fmt.Println("  syncver      Version synchronization")
	fmt.Println("  relcore      Manifest-driven release orchestration")
	fmt.Println("  waveprogress Regenerate wave tracking documents")
	fmt.Println("  addwave      Add a new staged-work wave (existing register items attach here)")
	fmt.Println("  gomod        go.mod/go.sum sanity gate")
	fmt.Println("  selftest     Acceptance gate")
	fmt.Println("\nTypical workflow:")
	fmt.Println("  1. Install the binary, then `repoman selftest` -- do not trust an")
	fmt.Println("     installation whose selftest fails.")
	fmt.Println("  2. Editing: `repoman ed find <term>` to see every occurrence and its")
	fmt.Println("     syntactic role, then `repoman ed apply`/`sub --expect N` -- never")
	fmt.Println("     hand-type an anchor you haven't seen. Mixed roles across the")
	fmt.Println("     matches: split into per-role passes, don't force one pass across")
	fmt.Println("     all of them.")
	fmt.Println("  3. Tracking open work: `repoman register add/list/close` against")
	fmt.Println("     docs/TRACKING.md.")
	fmt.Println("  4. Staged programmes: `repoman addwave` to define a wave (existing")
	fmt.Println("     register tickets attach via each item's register_item field --")
	fmt.Println("     there is no separate \"move a ticket into a wave\" command), then")
	fmt.Println("     `repoman waveprogress --show`/`--html` to render progress.")
	fmt.Println("  5. Releasing: `repoman syncver` to keep version strings in sync,")
	fmt.Println("     `repoman gomod check` for go.mod/go.sum sanity, then")
	fmt.Println("     `repoman relcore <version>` to orchestrate the release itself --")
	fmt.Println("     this runs `badcode check` unconditionally first, before anything")
	fmt.Println("     else, including on --resume.")
	fmt.Println("\nRun `repoman <command> -h` for help on a specific command -- every")
	fmt.Println("command's help links to the matching chapter of the full docs.")
	fmt.Println("Full documentation: https://ha1tch.github.io/gorepoman/")
	fmt.Println("(mirrors github.com/ha1tch/gorepoman -- try this URL if that one isn't")
	fmt.Println("reachable, or vice versa)")
}

// isHelpFlag matches every spelling of "I am asking for help" a
// person might reasonably try, confirmed as a real point of friction:
// a fresh session hit "unknown command" on help, --help, and -help in
// turn before eventually discovering -h worked -- but only once
// tried against a specific subcommand, not at the top level, since
// none of these four were recognized there at all. Every subcommand's
// own internal parser already correctly handles -h/--help; what was
// missing was top-level recognition of all four, and normalizing the
// two subcommand parsers don't already handle (help, -help) into one
// they do, so this fix lives in exactly one place rather than
// touching a dozen subcommand files individually.
func isHelpFlag(s string) bool {
	return s == "help" || s == "-h" || s == "--help" || s == "-help"
}

func main() {
	if len(os.Args) >= 2 && (os.Args[1] == "version" || os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Println("repoman " + version)
		os.Exit(0)
	}
	if len(os.Args) >= 2 && isHelpFlag(os.Args[1]) {
		printUsage()
		os.Exit(0)
	}
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]
	// A subcommand's own parser already recognizes -h and --help;
	// normalize the two spellings it doesn't (bare "help", and the
	// single-dash "-help") into one it does, so every subcommand gets
	// all four spellings without any change to its own logic.
	if len(args) > 0 && (args[0] == "help" || args[0] == "-help") {
		args[0] = "-h"
	}

	var exitCode int

	switch cmd {
	case "badcode":
		exitCode = badcode.Run(args)
	case "doctor":
		exitCode = doctor.Run(args)
	case "ed":
		exitCode = ed.Run(args)
	case "roles":
		exitCode = roles.Run(args)
	case "strreplace":
		exitCode = strreplace.Run(args)
	case "register":
		exitCode = register.Run(args)
	case "guards":
		exitCode = guards.Run(args)
	case "syncver":
		exitCode = syncver.Run(args)
	case "relcore":
		exitCode = relcore.Run(args)
	case "waveprogress":
		exitCode = waveprogress.Run(args)
	case "addwave":
		exitCode = addwave.Run(args)
	case "gomod":
		exitCode = gomod.Run(args)
	case "selftest":
		exitCode = selftest.Run(args)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", cmd)
		os.Exit(1)
	}

	os.Exit(exitCode)
}
