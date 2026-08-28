package main

import (
	"fmt"
	"os"

	"github.com/ha1tch/repoman/pkg/addwave"
	"github.com/ha1tch/repoman/pkg/doctor"
	"github.com/ha1tch/repoman/pkg/ed"
	"github.com/ha1tch/repoman/pkg/gomod"
	"github.com/ha1tch/repoman/pkg/guards"
	"github.com/ha1tch/repoman/pkg/register"
	"github.com/ha1tch/repoman/pkg/relcore"
	"github.com/ha1tch/repoman/pkg/roles"
	"github.com/ha1tch/repoman/pkg/selftest"
	"github.com/ha1tch/repoman/pkg/strreplace"
	"github.com/ha1tch/repoman/pkg/syncver"
	"github.com/ha1tch/repoman/pkg/waveprogress"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("repoman — repository-discipline tooling (Go translation)")
		fmt.Println("Usage: repoman <command> [args...]")
		fmt.Println("\nCommands:")
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
