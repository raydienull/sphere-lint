package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type lintError struct {
	file string
	line int
	kind string
	msg  string
}

type defLocation struct {
	file string
	line int
}

var (
	scriptsDir = "."
	extensions = []string{".scp"}
	ignoreDirs = map[string]bool{
		".git":    true,
		"backups": true,
		"backup":  true,
		"trash":   true,
		".github": true,
	}

	defHeaderPattern     = regexp.MustCompile(`^\[(\w+)\s+([^\]]+)\]`)
	commentHeaderPattern = regexp.MustCompile(`(?i)^\[COMMENT(?:\s+[^\]]+)?\]`)
	triggerPattern       = regexp.MustCompile(`(?i)^\s*ON\s*=\s*@?.+`)
	textLinePattern      = regexp.MustCompile(`^\s*[\w\d\.]*\b(SAY|SYSMESSAGE|MESSAGE|EMOTE|SAYU|SAYUA|TITLE|NAME|DESC|PROMPTCONSOLE|BARK|GROUP|EVENTS|FLAGS|RECT|P|AUTHOR|PAGES)\b`)

	commonErrors = []struct {
		pattern *regexp.Regexp
		msg     string
		kind    string
	}{
		{regexp.MustCompile(`^\s*DORAN\b`), "TYPO: 'DORAN' found. Did you mean 'DORAND'?", "TYPO"},
		{regexp.MustCompile(`^\s*EN\b`), "TYPO: 'EN' found. Did you mean 'ENDO', 'ENDDO', or 'ENDIF'?", "TYPO"},
		{regexp.MustCompile(`^\s*IF\s*$`), "LOGIC: empty 'IF' statement.", "LOGIC"},
		{regexp.MustCompile(`^\s*ELSEIF\s*$`), "LOGIC: empty 'ELSEIF' statement.", "LOGIC"},
		{regexp.MustCompile(`^\s*ELIF\s*$`), "LOGIC: empty 'ELIF' statement.", "LOGIC"},
		{regexp.MustCompile(`\[EOF\].+`), "CRITICAL: text found after [EOF].", "CRITICAL"},
	}

	blockStartToEnd = map[string]string{
		"IF":                "ENDIF",
		"WHILE":             "ENDWHILE",
		"FOR":               "ENDFOR",
		"FORCHARS":          "ENDFOR",
		"FORCHARMEMORYTYPE": "ENDFOR",
		"FORCONTTYPE":       "ENDFOR",
		"FORCHARLAYER":      "ENDFOR",
		"FORCLIENTS":        "ENDFOR",
		"FORITEMS":          "ENDFOR",
		"FOROBJS":           "ENDFOR",
		"FORCONT":           "ENDFOR",
		"FORCONTID":         "ENDFOR",
		"FORPLAYERS":        "ENDFOR",
		"FORINSTANCES":      "ENDFOR",
		"DORAND":            "ENDDO",
		"DOSWITCH":          "ENDDO",
		"DOSELECT":          "ENDDO",
		"BEGIN":             "END",
	}
)

func main() {
	defs := make(map[string]defLocation)
	var allErrors []lintError

	totalFiles := 0
	filesWithErrors := 0

	fmt.Println("=== SPHERE SCP LINT (Go Action) ===")

	err := filepath.WalkDir(scriptsDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			allErrors = append(allErrors, lintError{file: path, line: 1, kind: "CRITICAL", msg: walkErr.Error()})
			return nil
		}
		if d.IsDir() {
			if ignoreDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !hasExtension(path, extensions) {
			return nil
		}
		totalFiles++

		fileErrors := lintFile(path, defs)
		if len(fileErrors) > 0 {
			filesWithErrors++
			allErrors = append(allErrors, fileErrors...)
		}
		return nil
	})
	if err != nil {
		allErrors = append(allErrors, lintError{file: scriptsDir, line: 1, kind: "CRITICAL", msg: err.Error()})
	}

	for _, e := range allErrors {
		printAnnotation(e)
	}

	fmt.Println("---------------------------------------------")
	fmt.Printf("Files scanned: %d\n", totalFiles)
	fmt.Printf("Files with errors: %d\n", filesWithErrors)
	fmt.Printf("Total errors: %d\n", len(allErrors))

	if len(allErrors) > 0 {
		os.Exit(1)
	}
}

func lintFile(path string, defs map[string]defLocation) []lintError {
	var errors []lintError
	var stack []blockState
	inTextBlock := false

	rel := toRelative(path)

	file, err := os.Open(path)
	if err != nil {
		return []lintError{{file: rel, line: 1, kind: "CRITICAL", msg: err.Error()}}
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNum := 0
	lastNonEmpty := ""

	for scanner.Scan() {
		lineNum++
		raw := scanner.Text()
		cleaned := cleanLine(raw)
		if cleaned != "" {
			lastNonEmpty = cleaned
		}
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}
		if cleaned == "" {
			continue
		}

		if commentHeaderPattern.MatchString(cleaned) {
			if len(stack) > 0 {
				for _, b := range stack {
					errors = append(errors, lintError{file: rel, line: lineNum, kind: "BLOCK", msg: fmt.Sprintf("BLOCK: unclosed '%s' block before new section.", b.typ)})
				}
			}
			inTextBlock = true
			stack = nil
			continue
		}

		if defMatch := defHeaderPattern.FindStringSubmatch(cleaned); len(defMatch) == 3 {
			if len(stack) > 0 {
				for _, b := range stack {
					errors = append(errors, lintError{file: rel, line: lineNum, kind: "BLOCK", msg: fmt.Sprintf("BLOCK: unclosed '%s' block before new section.", b.typ)})
				}
			}
			defType := strings.ToUpper(defMatch[1])
			defArgs := strings.TrimSpace(defMatch[2])
			if defType == "BOOK" || defType == "COMMENT" {
				inTextBlock = true
			} else {
				inTextBlock = false
			}
			if defType == "ITEMDEF" || defType == "CHARDEF" || defType == "EVENTS" {
				id := strings.ToUpper(firstToken(defArgs))
				if id != "" {
					key := defType + " " + id
					if prev, ok := defs[key]; ok {
						errors = append(errors, lintError{
							file: rel,
							line: lineNum,
							kind: "DUPLICATE",
							msg:  fmt.Sprintf("DUPLICATE: '%s' already defined at %s:%d.", key, prev.file, prev.line),
						})
					} else {
						defs[key] = defLocation{file: rel, line: lineNum}
					}
				}
			}
			stack = nil
			continue
		}

		if triggerPattern.MatchString(cleaned) {
			inTextBlock = false
			if len(stack) > 0 {
				for _, b := range stack {
					errors = append(errors, lintError{file: rel, line: lineNum, kind: "BLOCK", msg: fmt.Sprintf("BLOCK: unclosed '%s' block before new trigger.", b.typ)})
				}
			}
			stack = nil
			continue
		}

		if inTextBlock {
			continue
		}

		upper := strings.ToUpper(cleaned)
		isWriteFile := strings.HasPrefix(upper, "SERV.WRITEFILE ")
		isTextLine := textLinePattern.MatchString(cleaned)
		isFlowControl := strings.HasPrefix(upper, "IF") || strings.HasPrefix(upper, "ELIF") || strings.HasPrefix(upper, "WHILE")
		isAssignment := strings.Contains(cleaned, "=") && !isFlowControl

		if !isTextLine && !isWriteFile {
			if bracketErr := checkBrackets(cleaned); bracketErr != "" {
				errors = append(errors, lintError{
					file: rel,
					line: lineNum,
					kind: "SYNTAX",
					msg:  "SYNTAX: brackets -> " + bracketErr,
				})
			}
		}

		if !isTextLine && !isAssignment {
			for _, ce := range commonErrors {
				if ce.pattern.MatchString(cleaned) {
					errors = append(errors, lintError{file: rel, line: lineNum, kind: ce.kind, msg: ce.msg})
				}
			}

			token := firstToken(upper)
			if token != "" {
				fields := strings.Fields(cleaned)
				if token == "WHILE" && len(fields) < 2 {
					errors = append(errors, lintError{file: rel, line: lineNum, kind: "LOGIC", msg: "LOGIC: WHILE missing condition."})
				}
				if token == "FOR" && len(fields) < 2 {
					errors = append(errors, lintError{file: rel, line: lineNum, kind: "LOGIC", msg: "LOGIC: FOR missing expression (expected: FOR <expr>, FOR <start> <end>, or FOR <var> <start> <end>)."})
				}
				if (token == "FORCHARS" || token == "FORITEMS" || token == "FOROBJS" || token == "FORCONT" || token == "FORCONTID" || token == "FORCONTTYPE" || token == "FORINSTANCES" || token == "FORCHARLAYER" || token == "FORCHARMEMORYTYPE") && len(fields) < 2 {
					errors = append(errors, lintError{file: rel, line: lineNum, kind: "LOGIC", msg: fmt.Sprintf("LOGIC: %s missing argument.", token)})
				}
				if token == "DORAND" && len(fields) < 2 {
					errors = append(errors, lintError{file: rel, line: lineNum, kind: "LOGIC", msg: "LOGIC: DORAND missing line count."})
				}
				if token == "DOSWITCH" && len(fields) < 2 {
					errors = append(errors, lintError{file: rel, line: lineNum, kind: "LOGIC", msg: "LOGIC: DOSWITCH missing line number."})
				}

				if endToken := normalizeEndToken(token); endToken != "" {
					if len(stack) == 0 {
						errors = append(errors, lintError{file: rel, line: lineNum, kind: "BLOCK", msg: fmt.Sprintf("BLOCK: '%s' without opening block.", token)})
					} else {
						last := stack[len(stack)-1]
						stack = stack[:len(stack)-1]
						expected := blockStartToEnd[last.typ]
						if endToken != expected {
							errors = append(errors, lintError{file: rel, line: lineNum, kind: "BLOCK", msg: fmt.Sprintf("BLOCK: mismatch. '%s' closed by '%s' (expected %s).", last.typ, token, expected)})
						}
					}
					continue
				}

				if token == "ELSE" || token == "ELIF" || token == "ELSEIF" {
					if len(stack) == 0 || stack[len(stack)-1].typ != "IF" {
						errors = append(errors, lintError{file: rel, line: lineNum, kind: "BLOCK", msg: fmt.Sprintf("BLOCK: '%s' without matching IF.", token)})
					}
					continue
				}

				if endToken := blockStartToEnd[token]; endToken != "" {
					stack = append(stack, blockState{typ: token, line: lineNum})
					continue
				}
			}
		}
	}

	if scanErr := scanner.Err(); scanErr != nil {
		errors = append(errors, lintError{file: rel, line: lineNum, kind: "CRITICAL", msg: scanErr.Error()})
	}

	if strings.ToUpper(strings.TrimSpace(lastNonEmpty)) != "[EOF]" {
		if lineNum == 0 {
			lineNum = 1
		}
		errors = append(errors, lintError{file: rel, line: lineNum, kind: "CRITICAL", msg: "CRITICAL: missing [EOF] at end of file."})
	}

	if len(stack) > 0 {
		for _, b := range stack {
			errors = append(errors, lintError{file: rel, line: b.line, kind: "BLOCK", msg: fmt.Sprintf("BLOCK: unclosed '%s' block.", b.typ)})
		}
	}

	return errors
}

type blockState struct {
	typ  string
	line int
}

func hasExtension(path string, exts []string) bool {
	for _, ext := range exts {
		if strings.HasSuffix(strings.ToLower(path), ext) {
			return true
		}
	}
	return false
}

func cleanLine(line string) string {
	if idx := strings.Index(line, "//"); idx >= 0 {
		line = line[:idx]
	}
	return strings.TrimSpace(line)
}

func firstToken(line string) string {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func normalizeEndToken(token string) string {
	switch token {
	case "ENDIF", "ENDWHILE", "ENDFOR", "ENDDO", "END":
		return token
	case "ENDO", "ENDOR":
		return "ENDDO"
	default:
		return ""
	}
}

func checkBrackets(line string) string {
	stack := make([]rune, 0, 8)
	pairs := map[rune]rune{')': '(', ']': '[', '}': '{'}
	for _, ch := range line {
		switch ch {
		case '(', '[', '{':
			stack = append(stack, ch)
		case ')', ']', '}':
			if len(stack) == 0 {
				return fmt.Sprintf("unexpected closing '%c'", ch)
			}
			expected := pairs[ch]
			if stack[len(stack)-1] != expected {
				return fmt.Sprintf("expected closing '%c' but found '%c'", stack[len(stack)-1], ch)
			}
			stack = stack[:len(stack)-1]
		}
	}
	if len(stack) > 0 {
		parts := make([]string, 0, len(stack))
		for _, ch := range stack {
			parts = append(parts, string(ch))
		}
		return "unclosed: " + strings.Join(parts, ", ")
	}
	return ""
}

func printAnnotation(e lintError) {
	msg := fmt.Sprintf("%s", e.msg)
	if e.line <= 0 {
		e.line = 1
	}
	fmt.Printf("::error file=%s,line=%d::%s\n", e.file, e.line, escapeAnnotation(msg))
}

func escapeAnnotation(msg string) string {
	replacer := strings.NewReplacer(
		"%", "%25",
		"\r", "%0D",
		"\n", "%0A",
	)
	return replacer.Replace(msg)
}

func toRelative(path string) string {
	rel, err := filepath.Rel(scriptsDir, path)
	if err != nil {
		return path
	}
	if rel == "." {
		return path
	}
	return filepath.ToSlash(rel)
}
