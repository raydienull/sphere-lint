package main

import (
	"fmt"
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

func TestLintUndefinedItemRef(t *testing.T) {
	content := strings.Join([]string{
		"[DEFNAME items_test]",
		"random_candy { i_missing_item 1 }",
		"[EOF]",
		"",
	}, "\n")

	errs := lintFromContent(t, "undefined_item_ref.scp", content)

	assertHasMessage(t, errs, "UNDECLARED: 'I_MISSING_ITEM' not defined as ITEMDEF")
}

func TestLintUndefinedSpawnRef(t *testing.T) {
	content := strings.Join([]string{
		"[DEFNAME spawns_test]",
		"random_spawn { spawn_missing_group 1 }",
		"[EOF]",
		"",
	}, "\n")

	errs := lintFromContent(t, "undefined_spawn_ref.scp", content)

	assertHasMessage(t, errs, "UNDECLARED: 'SPAWN_MISSING_GROUP' not defined as SPAWN")
}

func TestLintReferenceMatchesAnyDefID(t *testing.T) {
	content := strings.Join([]string{
		"[SPAWN c_08301]",
		"[DEFNAME spawns_test]",
		"random_spawn { c_08301 1 }",
		"[EOF]",
		"",
	}, "\n")

	errs := lintFromContent(t, "reference_any_def_id.scp", content)

	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %d", len(errs))
	}
}

func TestLintDefnameInsideMultidef(t *testing.T) {
	content := strings.Join([]string{
		"[MULTIDEF 01431]",
		"DEFNAME=m_foundation_12x16",
		"[DEFNAME menus_test]",
		"random_menu { m_foundation_12x16 1 }",
		"[EOF]",
		"",
	}, "\n")

	errs := lintFromContent(t, "defname_inside_multidef.scp", content)

	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %d", len(errs))
	}
}

func TestLintDynamicFunctionReference(t *testing.T) {
	content := strings.Join([]string{
		"[FUNCTION f_test]",
		"SERV.LOG <DEF.F_MULTIS_<SRC.CTAG0.ACCOUNTLANG>_MULTI_CENTER>",
		"[EOF]",
		"",
	}, "\n")

	errs := lintFromContent(t, "dynamic_function_ref.scp", content)

	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %d", len(errs))
	}
}

func TestLintItemDefnameAssignment(t *testing.T) {
	content := strings.Join([]string{
		"[ITEMDEF 03709]",
		"DEFNAME=i_fire_column",
		"[DEFNAME items_test]",
		"random_fx { i_fire_column 1 }",
		"[EOF]",
		"",
	}, "\n")

	errs := lintFromContent(t, "itemdef_defname.scp", content)

	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %d", len(errs))
	}
}

func TestLintResDefnameAliasReference(t *testing.T) {
	content := strings.Join([]string{
		"[RESDEFNAME backward_compatibility_defs]",
		"i_dragon_egg_lamp_s i_lamp_dragon_s",
		"[DEFNAME items_test]",
		"random_lamps { i_dragon_egg_lamp_s 1 }",
		"[EOF]",
		"",
	}, "\n")

	errs := lintFromContent(t, "resdefname_alias_ref.scp", content)

	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %d", len(errs))
	}
}

func TestLintResResDefnameAliasReference(t *testing.T) {
	content := strings.Join([]string{
		"[RES_RESDEFNAME backward_compatibility_defs]",
		"i_dragon_egg_lamp_s i_lamp_dragon_s",
		"[DEFNAME items_test]",
		"random_lamps { i_dragon_egg_lamp_s 1 }",
		"[EOF]",
		"",
	}, "\n")

	errs := lintFromContent(t, "res_resdefname_alias_ref.scp", content)

	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %d", len(errs))
	}
}

func TestLintOtherPrefixRefs(t *testing.T) {
	content := strings.Join([]string{
		"[FUNCTION f_test]",
		"RETURN 1",
		"[REGIONTYPE r_test]",
		"NAME=test",
		"[DEFNAME items_test]",
		"random_refs { f_test 1 r_test 1 }",
		"[EOF]",
		"",
	}, "\n")

	errs := lintFromContent(t, "other_prefix_refs.scp", content)

	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %d", len(errs))
	}
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

func TestLintAngleBracketMismatch(t *testing.T) {
	content := strings.Join([]string{
		"[ITEMDEF i_test]",
		"IF <SRC.NPC",
		"ENDIF",
		"[EOF]",
		"",
	}, "\n")

	errs := lintFromContent(t, "bad_angle_brackets.scp", content)

	assertHasMessage(t, errs, "SYNTAX: brackets")
}

func TestLintAngleBracketComparison(t *testing.T) {
	content := strings.Join([]string{
		"[ITEMDEF i_test]",
		"IF (<MOREY> > <MOREX>)",
		"ENDIF",
		"[EOF]",
		"",
	}, "\n")

	errs := lintFromContent(t, "angle_bracket_comparison.scp", content)

	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %d", len(errs))
	}
}

func TestLintEvalAngleExpressionComparison(t *testing.T) {
	lines := []string{
		"SRC.ACT.MOREY=<EVAL ((<SRC.KILLS> >= 3) || (<SRC.KARMA> < -1000) || (<SRC.FLAGS>&002000000))>",
		"VAR.TEST=<EVAL (<MOREY> > <MOREX>)>",
		"VAR.TEST=<EVAL (<MOREY> <= <MOREX>)>",
	}

	for i, line := range lines {
		content := strings.Join([]string{
			"[ITEMDEF i_test]",
			line,
			"[EOF]",
			"",
		}, "\n")

		errs := lintFromContent(t, fmt.Sprintf("eval_angle_expr_%d.scp", i), content)

		if len(errs) != 0 {
			t.Fatalf("expected no errors for case %d, got %d", i, len(errs))
		}
	}
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
			defIndex := map[string]definitionLocation{}
			defnameIndex := map[string]definitionLocation{}
			idIndex := map[string]definitionLocation{}
			var references []referenceUse
			contentA := buildDefContent(defType, "dup")
			contentB := buildDefContent(defType, "dup")

			pathA := writeTempFile(t, dir, "dup_"+strings.ToLower(defType)+"_a.scp", contentA)
			pathB := writeTempFile(t, dir, "dup_"+strings.ToLower(defType)+"_b.scp", contentB)

			assertNoErrors(t, lintScriptFile(pathA, defIndex, defnameIndex, idIndex, &references), "first "+defType+" def")
			assertHasMessage(t, lintScriptFile(pathB, defIndex, defnameIndex, idIndex, &references), "DUPLICATE: '"+defType+" DUP' already defined")
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

func lintFromContent(t *testing.T, name, content string) []lintIssue {
	t.Helper()
	dir := withTempScriptsDir(t)
	defIndex := map[string]definitionLocation{}
	defnameIndex := map[string]definitionLocation{}
	idIndex := map[string]definitionLocation{}
	var references []referenceUse

	path := writeTempFile(t, dir, name, content)
	errs := lintScriptFile(path, defIndex, defnameIndex, idIndex, &references)
	errs = append(errs, findUndefinedReferences(references, defIndex, defnameIndex, idIndex)...)
	return errs
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
	prevScriptsDir := scriptsRoot
	scriptsRoot = dir
	t.Cleanup(func() { scriptsRoot = prevScriptsDir })
	return dir
}

func assertNoErrors(t *testing.T, errs []lintIssue, context string) {
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

func assertHasMessage(t *testing.T, errs []lintIssue, needle string) {
	t.Helper()
	for _, e := range errs {
		if strings.Contains(e.msg, needle) {
			return
		}
	}
	t.Fatalf("expected error containing %q", needle)
}
