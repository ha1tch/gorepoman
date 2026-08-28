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

func main() {
	if len(os.Args) >= 2 && (os.Args[1] == "version" || os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Println("repoman " + version)
		os.Exit(0)
	}
	if len(os.Args) < 2 {
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
		fmt.Println("  addwave      Add a new staged-work wave")
		fmt.Println("  gomod        go.mod/go.sum sanity gate")
		fmt.Println("  selftest     Acceptance gate")
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

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
