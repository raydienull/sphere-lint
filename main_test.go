package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLintMissingLoopArgs(t *testing.T) {
	dir := t.TempDir()
	prevScriptsDir := scriptsDir
	scriptsDir = dir
	t.Cleanup(func() { scriptsDir = prevScriptsDir })

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

	path := writeTempFile(t, dir, "missing_loop_args.scp", content)
	errs := lintFile(path, map[string]defLocation{})

	assertHasMessage(t, errs, "LOGIC: FOR missing range")
	assertHasMessage(t, errs, "LOGIC: WHILE missing condition")
	assertHasMessage(t, errs, "LOGIC: DORAND missing line count")
	assertHasMessage(t, errs, "LOGIC: DOSWITCH missing line number")
	assertHasMessage(t, errs, "LOGIC: FOROBJS missing argument")
}

func TestLintForRangeWithoutVar(t *testing.T) {
	dir := t.TempDir()
	prevScriptsDir := scriptsDir
	scriptsDir = dir
	t.Cleanup(func() { scriptsDir = prevScriptsDir })

	content := strings.Join([]string{
		"[ITEMDEF i_ok]",
		"FOR 1 3",
		"ENDFOR",
		"[EOF]",
		"",
	}, "\n")

	path := writeTempFile(t, dir, "for_no_var.scp", content)
	errs := lintFile(path, map[string]defLocation{})

	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %d", len(errs))
	}
}

func TestLintTriggerWithSpaceFlushesBlocks(t *testing.T) {
	dir := t.TempDir()
	prevScriptsDir := scriptsDir
	scriptsDir = dir
	t.Cleanup(func() { scriptsDir = prevScriptsDir })

	content := strings.Join([]string{
		"[ITEMDEF i_test]",
		"ON=@Create",
		"IF <SRC.NPC>",
		"ON=@DropOn Char",
		"[EOF]",
		"",
	}, "\n")

	path := writeTempFile(t, dir, "trigger_space.scp", content)
	errs := lintFile(path, map[string]defLocation{})

	assertHasMessage(t, errs, "BLOCK: unclosed 'IF' block before new trigger.")
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
