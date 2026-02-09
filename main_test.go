package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLintMissingLoopArgs(t *testing.T) {
	content := strings.Join([]string{
		"[ITEMDEF i_test]",
		"ON=@Create",
		"FOR",
		"ENDFOR",
		"WHILE",
		"ENDWHILE",
		"DORAND",
		"ENDDO",
		"DOSWITCH",
		"ENDDO",
		"FOROBJS",
		"ENDFOR",
		"[EOF]",
		"",
	}, "\n")

	errs := lintFromContent(t, "missing_loop_args.scp", content)

	assertHasMessage(t, errs, "LOGIC: FOR missing expression")
	assertHasMessage(t, errs, "LOGIC: WHILE missing condition")
	assertHasMessage(t, errs, "LOGIC: DORAND missing line count")
	assertHasMessage(t, errs, "LOGIC: DOSWITCH missing line number")
	assertHasMessage(t, errs, "LOGIC: FOROBJS missing argument")
}

func TestLintForRangeWithoutVar(t *testing.T) {
	content := strings.Join([]string{
		"[ITEMDEF i_ok]",
		"FOR 1 3",
		"ENDFOR",
		"[EOF]",
		"",
	}, "\n")

	errs := lintFromContent(t, "for_no_var.scp", content)

	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %d", len(errs))
	}
}

func TestLintTriggerWithSpaceFlushesBlocks(t *testing.T) {
	content := strings.Join([]string{
		"[ITEMDEF i_test]",
		"ON=@Create",
		"IF <SRC.NPC>",
		"ON=@DropOn Char",
		"[EOF]",
		"",
	}, "\n")

	errs := lintFromContent(t, "trigger_space.scp", content)

	assertHasMessage(t, errs, "BLOCK: unclosed 'IF' block before new trigger.")
}

func TestLintMissingEOF(t *testing.T) {
	content := strings.Join([]string{
		"[ITEMDEF i_test]",
		"ON=@Create",
		"IF 1",
		"ENDIF",
		"",
	}, "\n")

	errs := lintFromContent(t, "missing_eof.scp", content)

	assertHasMessage(t, errs, "CRITICAL: missing [EOF] at end of file.")
}

func TestLintTextAfterEOF(t *testing.T) {
	content := strings.Join([]string{
		"[ITEMDEF i_test]",
		"[EOF] trailing",
		"",
	}, "\n")

	errs := lintFromContent(t, "text_after_eof.scp", content)

	assertHasMessage(t, errs, "CRITICAL: text found after [EOF].")
}

func TestLintBracketMismatch(t *testing.T) {
	content := strings.Join([]string{
		"[ITEMDEF i_test]",
		"IF (1",
		"ENDIF",
		"[EOF]",
		"",
	}, "\n")

	errs := lintFromContent(t, "bad_brackets.scp", content)

	assertHasMessage(t, errs, "SYNTAX: brackets")
}

func TestLintDuplicateDefsAcrossFiles(t *testing.T) {
	dir := withTempScriptsDir(t)
	defTypes := []string{
		"ITEMDEF",
		"CHARDEF",
		"EVENTS",
		"FUNCTION",
		"REGIONTYPE",
		"AREADEF",
		"DIALOG",
		"MENU",
		"ROOMDEF",
		"SKILL",
		"SKILLCLASS",
		"SKILLMENU",
		"SPAWN",
		"SPELL",
		"TYPEDEF",
	}

	for _, defType := range defTypes {
		defType := defType
		t.Run(defType, func(t *testing.T) {
			defs := map[string]defLocation{}
			contentA := buildDefContent(defType, "dup")
			contentB := buildDefContent(defType, "dup")

			pathA := writeTempFile(t, dir, "dup_"+strings.ToLower(defType)+"_a.scp", contentA)
			pathB := writeTempFile(t, dir, "dup_"+strings.ToLower(defType)+"_b.scp", contentB)

			assertNoErrors(t, lintFile(pathA, defs), "first "+defType+" def")
			assertHasMessage(t, lintFile(pathB, defs), "DUPLICATE: '"+defType+" DUP' already defined")
		})
	}
}

func TestLintDialogDuplicateWithSections(t *testing.T) {
	content := strings.Join([]string{
		"[DIALOG dialog]",
		"[DIALOG dialog TEXT]",
		"[DIALOG dialog BUTTON]",
		"[DIALOG dialog TEXT]",
		"[EOF]",
		"",
	}, "\n")

	errs := lintFromContent(t, "dialog_sections.scp", content)

	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	assertHasMessage(t, errs, "DUPLICATE: 'DIALOG DIALOG TEXT' already defined")
}

func lintFromContent(t *testing.T, name, content string) []lintError {
	t.Helper()
	dir := withTempScriptsDir(t)

	path := writeTempFile(t, dir, name, content)
	return lintFile(path, map[string]defLocation{})
}

func writeTempFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	return path
}

func buildDefContent(defType, id string) string {
	return strings.Join([]string{
		"[" + defType + " " + id + "]",
		"[EOF]",
		"",
	}, "\n")
}

func withTempScriptsDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	prevScriptsDir := scriptsDir
	scriptsDir = dir
	t.Cleanup(func() { scriptsDir = prevScriptsDir })
	return dir
}

func assertNoErrors(t *testing.T, errs []lintError, context string) {
	t.Helper()
	if len(errs) == 0 {
		return
	}
	parts := make([]string, 0, len(errs))
	for _, e := range errs {
		parts = append(parts, e.msg)
	}
	t.Fatalf("expected no errors for %s, got %d: %s", context, len(errs), strings.Join(parts, " | "))
}

func assertHasMessage(t *testing.T, errs []lintError, needle string) {
	t.Helper()
	for _, e := range errs {
		if strings.Contains(e.msg, needle) {
			return
		}
	}
	t.Fatalf("expected error containing %q", needle)
}
