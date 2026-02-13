package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLintMissingLoopArgs(t *testing.T) {
	content := joinLines(
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
	)

	errs := lintFromContent(t, "missing_loop_args.scp", content)

	assertHasMessage(t, errs, "LOGIC: FOR missing expression")
	assertHasMessage(t, errs, "LOGIC: WHILE missing condition")
	assertHasMessage(t, errs, "LOGIC: DORAND missing line count")
	assertHasMessage(t, errs, "LOGIC: DOSWITCH missing line number")
	assertHasMessage(t, errs, "LOGIC: FOROBJS missing argument")
}

func TestLintLogicErrors(t *testing.T) {
	t.Run("ForRangeWithoutVar", func(t *testing.T) {
		content := joinLines(
			"[ITEMDEF i_ok]",
			"FOR 1 3",
			"ENDFOR",
			"[EOF]",
		)

		assertNoErrors(t, lintFromContent(t, "for_no_var.scp", content), "for range without var")
	})
}

func TestLintBlockErrors(t *testing.T) {
	t.Run("TriggerStartsNewBlock", func(t *testing.T) {
		content := joinLines(
			"[ITEMDEF i_test]",
			"ON=@Create",
			"IF <SRC.NPC>",
			"ON=@DropOn Char",
			"[EOF]",
		)

		errs := lintFromContent(t, "trigger_space.scp", content)

		assertHasMessage(t, errs, "BLOCK: unclosed 'IF' block before new trigger.")
	})
}

func TestLintCriticalErrors(t *testing.T) {
	t.Run("MissingEOF", func(t *testing.T) {
		content := joinLines(
			"[ITEMDEF i_test]",
			"ON=@Create",
			"IF 1",
			"ENDIF",
		)

		errs := lintFromContent(t, "missing_eof.scp", content)

		assertHasMessage(t, errs, "CRITICAL: missing [EOF] at end of file.")
	})

	t.Run("TextAfterEOF", func(t *testing.T) {
		content := joinLines(
			"[ITEMDEF i_test]",
			"[EOF] trailing",
		)

		errs := lintFromContent(t, "text_after_eof.scp", content)

		assertHasMessage(t, errs, "CRITICAL: text found after [EOF].")
	})
}

func TestLintReferenceErrors(t *testing.T) {
	t.Run("UndefinedReferences", func(t *testing.T) {
		itemContent := joinLines(
			"[DEFNAME items_test]",
			"random_candy { i_missing_item 1 }",
			"[EOF]",
		)

		itemErrs := lintFromContent(t, "undefined_item_ref.scp", itemContent)
		assertHasMessage(t, itemErrs, "UNDECLARED: 'I_MISSING_ITEM' not defined as ITEMDEF")

		spawnContent := joinLines(
			"[DEFNAME spawns_test]",
			"random_spawn { spawn_missing_group 1 }",
			"[EOF]",
		)

		spawnErrs := lintFromContent(t, "undefined_spawn_ref.scp", spawnContent)
		assertHasMessage(t, spawnErrs, "UNDECLARED: 'SPAWN_MISSING_GROUP' not defined as SPAWN")
	})

	t.Run("ValidReferences", func(t *testing.T) {
		cases := []struct {
			name    string
			content string
		}{
			{
				name: "ReferenceMatchesAnyDefID",
				content: joinLines(
					"[SPAWN c_08301]",
					"[DEFNAME spawns_test]",
					"random_spawn { c_08301 1 }",
					"[EOF]",
				),
			},
			{
				name: "DefnameInsideMultidef",
				content: joinLines(
					"[MULTIDEF 01431]",
					"DEFNAME=m_foundation_12x16",
					"[DEFNAME menus_test]",
					"random_menu { m_foundation_12x16 1 }",
					"[EOF]",
				),
			},
			{
				name: "DynamicFunctionReference",
				content: joinLines(
					"[FUNCTION f_test]",
					"SERV.LOG <DEF.F_MULTIS_<SRC.CTAG0.ACCOUNTLANG>_MULTI_CENTER>",
					"[EOF]",
				),
			},
			{
				name: "ItemDefnameAssignment",
				content: joinLines(
					"[ITEMDEF 03709]",
					"DEFNAME=i_fire_column",
					"[DEFNAME items_test]",
					"random_fx { i_fire_column 1 }",
					"[EOF]",
				),
			},
			{
				name: "ResdefnameAliasReference",
				content: joinLines(
					"[RESDEFNAME backward_compatibility_defs]",
					"i_dragon_egg_lamp_s i_lamp_dragon_s",
					"[DEFNAME items_test]",
					"random_lamps { i_dragon_egg_lamp_s 1 }",
					"[EOF]",
				),
			},
			{
				name: "ResResdefnameAliasReference",
				content: joinLines(
					"[RES_RESDEFNAME backward_compatibility_defs]",
					"i_dragon_egg_lamp_s i_lamp_dragon_s",
					"[DEFNAME items_test]",
					"random_lamps { i_dragon_egg_lamp_s 1 }",
					"[EOF]",
				),
			},
			{
				name: "OtherPrefixRefs",
				content: joinLines(
					"[FUNCTION f_test]",
					"RETURN 1",
					"[REGIONTYPE r_test]",
					"NAME=test",
					"[DEFNAME items_test]",
					"random_refs { f_test 1 r_test 1 }",
					"[EOF]",
				),
			},
		}

		for _, tc := range cases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				assertNoErrors(t, lintFromContent(t, strings.ToLower(tc.name)+".scp", tc.content), tc.name)
			})
		}
	})
}

func TestLintSyntaxErrors(t *testing.T) {
	t.Run("InvalidBrackets", func(t *testing.T) {
		cases := []struct {
			name string
			line string
		}{
			{name: "ParenMismatch", line: "IF (1"},
			{name: "AngleMismatch", line: "IF <SRC.NPC"},
		}

		for _, tc := range cases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				content := joinLines(
					"[ITEMDEF i_test]",
					tc.line,
					"ENDIF",
					"[EOF]",
				)

				errs := lintFromContent(t, strings.ToLower(tc.name)+".scp", content)
				assertHasMessage(t, errs, "SYNTAX: brackets")
			})
		}
	})

	t.Run("AngleComparisons", func(t *testing.T) {
		content := joinLines(
			"[ITEMDEF i_test]",
			"IF (<MOREY> > <MOREX>)",
			"ENDIF",
			"[EOF]",
		)

		assertNoErrors(t, lintFromContent(t, "angle_bracket_comparison.scp", content), "angle bracket comparison")
	})

	t.Run("EvalAngleExpressions", func(t *testing.T) {
		lines := []string{
			"SRC.ACT.MOREY=<EVAL ((<SRC.KILLS> >= 3) || (<SRC.KARMA> < -1000) || (<SRC.FLAGS>&002000000))>",
			"LOCAL.TEST=<EVAL (<MORE>)>/8",
			"LOCAL.TEST2=<EVAL (<MORE>)</8",
			"VAR.TEST=<EVAL (<MOREY> > <MOREX>)>",
			"VAR.TEST=<EVAL (<MOREY> <= <MOREX>)>",
		}

		for i, line := range lines {
			content := joinLines(
				"[ITEMDEF i_test]",
				line,
				"[EOF]",
			)

			assertNoErrors(t, lintFromContent(t, fmt.Sprintf("eval_angle_expr_%d.scp", i), content), fmt.Sprintf("eval angle case %d", i))
		}
	})
}

func TestLintDuplicateDefinitions(t *testing.T) {
	t.Run("AcrossFiles", func(t *testing.T) {
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
	})

	t.Run("DialogSections", func(t *testing.T) {
		content := joinLines(
			"[DIALOG dialog]",
			"[DIALOG dialog TEXT]",
			"[DIALOG dialog BUTTON]",
			"[DIALOG dialog TEXT]",
			"[EOF]",
		)

		errs := lintFromContent(t, "dialog_sections.scp", content)

		if len(errs) != 1 {
			t.Fatalf("expected 1 error, got %d", len(errs))
		}
		assertHasMessage(t, errs, "DUPLICATE: 'DIALOG DIALOG TEXT' already defined")
	})
}

func TestLintTemplateChecks(t *testing.T) {
	t.Run("ValidTemplateReferences", func(t *testing.T) {
		content := joinLines(
			"[TEMPLATE loot_pack]",
			"CONTAINER=i_backpack",
			"ITEM=random_food",
			"ITEM=i_gold",
			"[TEMPLATE random_food]",
			"ITEM=i_apple",
			"[ITEMDEF i_backpack]",
			"[ITEMDEF i_gold]",
			"[ITEMDEF i_apple]",
			"[EOF]",
		)

		assertNoErrors(t, lintFromContent(t, "template_valid.scp", content), "valid template refs")
	})

	t.Run("TemplateRangeSpacing", func(t *testing.T) {
		content := joinLines(
			"[TEMPLATE loot_pack]",
			"ITEM={ 1 3 }",
			"[EOF]",
		)

		errs := lintFromContent(t, "template_range_spacing.scp", content)
		assertHasMessage(t, errs, "SYNTAX: template range selector")
	})

	t.Run("TemplateRangeCount", func(t *testing.T) {
		content := joinLines(
			"[TEMPLATE loot_pack]",
			"ITEM={1 2 3}",
			"[EOF]",
		)

		errs := lintFromContent(t, "template_range_count.scp", content)
		assertHasMessage(t, errs, "SYNTAX: template range selector")
	})

	t.Run("TemplateRSelector", func(t *testing.T) {
		content := joinLines(
			"[TEMPLATE loot_pack]",
			"ITEM=i_sword_long,R1A",
			"[EOF]",
		)

		errs := lintFromContent(t, "template_r_selector.scp", content)
		assertHasMessage(t, errs, "SYNTAX: template R selector")
	})

	t.Run("TemplateEmptyItem", func(t *testing.T) {
		content := joinLines(
			"[TEMPLATE loot_pack]",
			"ITEM=",
			"[EOF]",
		)

		errs := lintFromContent(t, "template_empty_item.scp", content)
		assertHasMessage(t, errs, "LOGIC: ITEM missing value")
	})

	t.Run("TemplateEmptyContainer", func(t *testing.T) {
		content := joinLines(
			"[TEMPLATE loot_pack]",
			"CONTAINER=",
			"[EOF]",
		)

		errs := lintFromContent(t, "template_empty_container.scp", content)
		assertHasMessage(t, errs, "LOGIC: CONTAINER missing value")
	})

	t.Run("UndefinedTemplateItem", func(t *testing.T) {
		content := joinLines(
			"[TEMPLATE loot_pack]",
			"ITEM=random_missing",
			"[EOF]",
		)

		errs := lintFromContent(t, "template_missing_item.scp", content)
		assertHasMessage(t, errs, "UNDECLARED: 'RANDOM_MISSING' not defined as ITEMDEF/TEMPLATE")
	})

	t.Run("UndefinedContainerItem", func(t *testing.T) {
		content := joinLines(
			"[TEMPLATE loot_pack]",
			"CONTAINER=i_missing",
			"[EOF]",
		)

		errs := lintFromContent(t, "template_missing_container.scp", content)
		assertHasMessage(t, errs, "UNDECLARED: 'I_MISSING' not defined as ITEMDEF")
	})

	t.Run("TemplateBrackets", func(t *testing.T) {
		content := joinLines(
			"[TEMPLATE loot_pack]",
			"ITEM={ random_food 1 0 3",
			"[EOF]",
		)

		errs := lintFromContent(t, "template_bad_brackets.scp", content)
		assertHasMessage(t, errs, "SYNTAX: brackets")
	})
}

func TestLintCommentSection(t *testing.T) {
	content := joinLines(
		"[COMMENT sphere_newb]",
		"If the player choose an race which has no template set, the",
		"default human template will be used by default.",
		"  [NEWBIE Alchemy]",
		"  IF 1",
		"[ITEMDEF i_test]",
		"[EOF]",
	)

	assertNoErrors(t, lintFromContent(t, "comment_section.scp", content), "comment section")
}

func joinLines(lines ...string) string {
	return strings.Join(append(lines, ""), "\n")
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
