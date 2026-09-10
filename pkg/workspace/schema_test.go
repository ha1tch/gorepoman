package workspace

import "testing"

func TestValidateBoard_DependencyOrderRejectsValues(t *testing.T) {
	b := Board{
		Schema: "gorepoman.workspace.board/1", Name: "b", Axis: "dependency_order",
		Columns: []BoardColumn{{Label: "Now", Values: []string{"☐"}}},
	}
	if err := ValidateBoard(b); err == nil {
		t.Fatal("expected an error: dependency_order columns must not carry values")
	}
}

func TestValidateBoard_DependencyOrderWithoutValuesOK(t *testing.T) {
	b := Board{
		Schema: "gorepoman.workspace.board/1", Name: "b", Axis: "dependency_order",
		Columns: []BoardColumn{{Label: "Now"}, {Label: "Next"}},
	}
	if err := ValidateBoard(b); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestValidateBoard_ThemeRequiresValues(t *testing.T) {
	b := Board{
		Schema: "gorepoman.workspace.board/1", Name: "b", Axis: "theme",
		Columns: []BoardColumn{{Label: "Core"}},
	}
	if err := ValidateBoard(b); err == nil {
		t.Fatal("expected an error: theme columns must carry values, there is no natural order to fall back on")
	}
}

func TestValidateBoard_ThemeWithValuesOK(t *testing.T) {
	b := Board{
		Schema: "gorepoman.workspace.board/1", Name: "b", Axis: "theme",
		Columns: []BoardColumn{
			{Label: "Core", Values: []string{"core", "durability"}},
			{Label: "Other", Values: []string{"dx", "cli"}},
		},
	}
	if err := ValidateBoard(b); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestValidateBoard_StatusValuesOptional(t *testing.T) {
	b := Board{
		Schema: "gorepoman.workspace.board/1", Name: "b", Axis: "status",
		Columns: []BoardColumn{{Label: "Not started"}, {Label: "Done"}},
	}
	if err := ValidateBoard(b); err != nil {
		t.Fatalf("status columns without values should fall back to natural order, got: %v", err)
	}
}

func TestValidateBoard_DuplicateValueAcrossColumnsRejected(t *testing.T) {
	b := Board{
		Schema: "gorepoman.workspace.board/1", Name: "b", Axis: "status",
		Columns: []BoardColumn{
			{Label: "A", Values: []string{"☑", "✓"}},
			{Label: "B", Values: []string{"✓"}},
		},
	}
	if err := ValidateBoard(b); err == nil {
		t.Fatal("expected an error: ✓ claimed by two columns")
	}
}

func TestValidateBoard_NoDuplicateAcrossColumnsOK(t *testing.T) {
	b := Board{
		Schema: "gorepoman.workspace.board/1", Name: "b", Axis: "priority",
		Columns: []BoardColumn{
			{Label: "Urgent", Values: []string{"P1"}},
			{Label: "Rest", Values: []string{"P2", "P3"}},
		},
	}
	if err := ValidateBoard(b); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}
