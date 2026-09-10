package gomod

// CheckResult is the structured shape of what `check` reports --
// computeCheck below duplicates cmdCheck's own logic to build this,
// rather than modifying cmdCheck itself, so the default text output
// (which cmdCheck still produces directly) is provably unchanged.
type CheckResult struct {
	ToolchainAvailable bool     `json:"toolchain_available"`
	Errors             []string `json:"errors"`
	Warnings           []string `json:"warnings"`
	OK                 bool     `json:"ok"`
}

// computeCheck mirrors cmdCheck's own logic exactly, but collects
// results instead of printing them. cmdCheck itself is untouched --
// this is new code alongside it, not a refactor of it.
func computeCheck(root string, strictRelative bool) CheckResult {
	res := run([]string{"go", "version"}, root)
	if res.rc == nil {
		return CheckResult{ToolchainAvailable: false, Errors: []string{}, Warnings: []string{}, OK: true}
	}

	var errors, warnings []string
	e, w := checkReplaceDirectives(root, strictRelative)
	errors = append(errors, e...)
	warnings = append(warnings, w...)
	e, w = checkGosumCompleteness(root)
	errors = append(errors, e...)
	warnings = append(warnings, w...)
	if errors == nil {
		errors = []string{}
	}
	if warnings == nil {
		warnings = []string{}
	}

	return CheckResult{
		ToolchainAvailable: true,
		Errors:             errors,
		Warnings:           warnings,
		OK:                 len(errors) == 0,
	}
}
