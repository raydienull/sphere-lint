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
	dir := t.TempDir()
	prevScriptsDir := scriptsDir
	scriptsDir = dir
	t.Cleanup(func() { scriptsDir = prevScriptsDir })

	defs := map[string]defLocation{}
	contentA := strings.Join([]string{
		"[ITEMDEF i_dup]",
		"[EOF]",
		"",
	}, "\n")
	contentB := strings.Join([]string{
		"[ITEMDEF i_dup]",
		"[EOF]",
		"",
	}, "\n")

	pathA := writeTempFile(t, dir, "dup_a.scp", contentA)
	pathB := writeTempFile(t, dir, "dup_b.scp", contentB)

	first := lintFile(pathA, defs)
	if len(first) != 0 {
		t.Fatalf("expected no errors for first def, got %d", len(first))
	}

	second := lintFile(pathB, defs)
	assertHasMessage(t, second, "DUPLICATE: 'ITEMDEF I_DUP' already defined")
}

func lintFromContent(t *testing.T, name, content string) []lintError {
	t.Helper()
	dir := t.TempDir()
	prevScriptsDir := scriptsDir
	scriptsDir = dir
	t.Cleanup(func() { scriptsDir = prevScriptsDir })

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

func assertHasMessage(t *testing.T, errs []lintError, needle string) {
	t.Helper()
	for _, e := range errs {
		if strings.Contains(e.msg, needle) {
			return
		}
	}
	t.Fatalf("expected error containing %q", needle)
}
