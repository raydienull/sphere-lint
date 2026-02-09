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

	textKeywords = map[string]bool{
		"SAY": true, "SYSMESSAGE": true, "MESSAGE": true, "EMOTE": true, "SAYU": true, "SAYUA": true,
		"TITLE": true, "NAME": true, "DESC": true, "PROMPTCONSOLE": true, "BARK": true, "GROUP": true,
		"EVENTS": true, "FLAGS": true, "RECT": true, "P": true, "AUTHOR": true, "PAGES": true,
	}

	bracketPairs = map[rune]rune{')': '(', ']': '[', '}': '{'}

	missingArgMessages = map[string]string{
		"WHILE":    "LOGIC: WHILE missing condition.",
		"FOR":      "LOGIC: FOR missing expression (expected: FOR <expr>, FOR <start> <end>, or FOR <var> <start> <end>).",
		"DORAND":   "LOGIC: DORAND missing line count.",
		"DOSWITCH": "LOGIC: DOSWITCH missing line number.",
	}

	forArgTokens = map[string]bool{
		"FORCHARS":          true,
		"FORITEMS":          true,
		"FOROBJS":           true,
		"FORCONT":           true,
		"FORCONTID":         true,
		"FORCONTTYPE":       true,
		"FORINSTANCES":      true,
		"FORCHARLAYER":      true,
		"FORCHARMEMORYTYPE": true,
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
		printError(e)
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
		if cleaned == "" {
			continue
		}

		if commentHeaderPattern.MatchString(cleaned) {
			errors = appendUnclosedStackErrors(errors, stack, rel, lineNum, " before new section.", false)
			inTextBlock = true
			stack = nil
			continue
		}

		if defMatch := defHeaderPattern.FindStringSubmatch(cleaned); len(defMatch) == 3 {
			errors = appendUnclosedStackErrors(errors, stack, rel, lineNum, " before new section.", false)
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
			errors = appendUnclosedStackErrors(errors, stack, rel, lineNum, " before new trigger.", false)
			stack = nil
			continue
		}

		if inTextBlock {
			continue
		}

		token := firstToken(cleaned)
		upperToken := strings.ToUpper(token)
		isWriteFile := hasPrefixFold(cleaned, "SERV.WRITEFILE ")
		isTextLine := isTextKeyword(token)
		isFlowControl := upperToken == "IF" || upperToken == "ELIF" || upperToken == "ELSEIF" || upperToken == "WHILE"
		isAssignment := strings.Contains(cleaned, "=") && !isFlowControl

		if !isTextLine && !isWriteFile {
			if bracketErr := checkBrackets(cleaned); bracketErr != "" {
				errors = appendError(errors, rel, lineNum, "SYNTAX", "SYNTAX: brackets -> "+bracketErr)
			}
		}

		if !isTextLine && !isAssignment {
			if upperToken == "DORAN" {
				errors = appendError(errors, rel, lineNum, "TYPO", "TYPO: 'DORAN' found. Did you mean 'DORAND'?")
			}
			if upperToken == "EN" {
				errors = appendError(errors, rel, lineNum, "TYPO", "TYPO: 'EN' found. Did you mean 'ENDO', 'ENDDO', or 'ENDIF'?")
			}
			if upperToken == "IF" && strings.TrimSpace(cleaned) == "IF" {
				errors = appendError(errors, rel, lineNum, "LOGIC", "LOGIC: empty 'IF' statement.")
			}
			if upperToken == "ELSEIF" && strings.TrimSpace(cleaned) == "ELSEIF" {
				errors = appendError(errors, rel, lineNum, "LOGIC", "LOGIC: empty 'ELSEIF' statement.")
			}
			if upperToken == "ELIF" && strings.TrimSpace(cleaned) == "ELIF" {
				errors = appendError(errors, rel, lineNum, "LOGIC", "LOGIC: empty 'ELIF' statement.")
			}
			trimmed := strings.TrimSpace(cleaned)
			if strings.HasPrefix(trimmed, "[EOF]") && trimmed != "[EOF]" {
				errors = appendError(errors, rel, lineNum, "CRITICAL", "CRITICAL: text found after [EOF].")
			}

			if upperToken != "" {
				fieldCount := countFields(cleaned, 2)
				if fieldCount < 2 {
					if msg, ok := missingArgMessages[upperToken]; ok {
						errors = appendError(errors, rel, lineNum, "LOGIC", msg)
					} else if forArgTokens[upperToken] {
						errors = appendError(errors, rel, lineNum, "LOGIC", fmt.Sprintf("LOGIC: %s missing argument.", upperToken))
					}
				}

				if endToken := normalizeEndToken(upperToken); endToken != "" {
					if len(stack) == 0 {
						errors = appendError(errors, rel, lineNum, "BLOCK", fmt.Sprintf("BLOCK: '%s' without opening block.", upperToken))
					} else {
						last := stack[len(stack)-1]
						stack = stack[:len(stack)-1]
						expected := blockStartToEnd[last.typ]
						if endToken != expected {
							errors = appendError(errors, rel, lineNum, "BLOCK", fmt.Sprintf("BLOCK: mismatch. '%s' closed by '%s' (expected %s).", last.typ, upperToken, expected))
						}
					}
					continue
				}

				if upperToken == "ELSE" || upperToken == "ELIF" || upperToken == "ELSEIF" {
					if len(stack) == 0 || stack[len(stack)-1].typ != "IF" {
						errors = appendError(errors, rel, lineNum, "BLOCK", fmt.Sprintf("BLOCK: '%s' without matching IF.", upperToken))
					}
					continue
				}

				if endToken := blockStartToEnd[upperToken]; endToken != "" {
					stack = append(stack, blockState{typ: upperToken, line: lineNum})
					continue
				}
			}
		}
	}

	if scanErr := scanner.Err(); scanErr != nil {
		errors = appendError(errors, rel, lineNum, "CRITICAL", scanErr.Error())
	}

	if strings.ToUpper(strings.TrimSpace(lastNonEmpty)) != "[EOF]" {
		if lineNum == 0 {
			lineNum = 1
		}
		errors = appendError(errors, rel, lineNum, "CRITICAL", "CRITICAL: missing [EOF] at end of file.")
	}

	if len(stack) > 0 {
		errors = appendUnclosedStackErrors(errors, stack, rel, lineNum, ".", true)
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
	for _, ch := range line {
		switch ch {
		case '(', '[', '{':
			stack = append(stack, ch)
		case ')', ']', '}':
			if len(stack) == 0 {
				return fmt.Sprintf("unexpected closing '%c'", ch)
			}
			expected := bracketPairs[ch]
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

func printError(e lintError) {
	if e.line <= 0 {
		e.line = 1
	}
	if isGitHubActions() {
		msg := e.msg
		if e.file != "" {
			msg = fmt.Sprintf("%s:%d: %s", e.file, e.line, msg)
		}
		fmt.Printf("::error file=%s,line=%d::%s\n", e.file, e.line, escapeAnnotation(msg))
		return
	}
	if e.file != "" {
		fmt.Printf("ERROR %s:%d: %s\n", e.file, e.line, e.msg)
		return
	}
	fmt.Printf("ERROR %s\n", e.msg)
}

func isGitHubActions() bool {
	return os.Getenv("GITHUB_ACTIONS") == "true"
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

func appendUnclosedStackErrors(errors []lintError, stack []blockState, rel string, lineNum int, msgSuffix string, useBlockLine bool) []lintError {
	if len(stack) == 0 {
		return errors
	}
	for _, b := range stack {
		errLine := lineNum
		if useBlockLine {
			errLine = b.line
		}
		errors = append(errors, lintError{file: rel, line: errLine, kind: "BLOCK", msg: fmt.Sprintf("BLOCK: unclosed '%s' block%s", b.typ, msgSuffix)})
	}
	return errors
}

func appendError(errors []lintError, rel string, lineNum int, kind, msg string) []lintError {
	return append(errors, lintError{file: rel, line: lineNum, kind: kind, msg: msg})
}

func countFields(line string, max int) int {
	count := 0
	inField := false
	for i := 0; i < len(line); i++ {
		if line[i] == ' ' || line[i] == '\t' || line[i] == '\r' || line[i] == '\n' {
			if inField {
				inField = false
			}
			continue
		}
		if !inField {
			count++
			if max > 0 && count >= max {
				return count
			}
			inField = true
		}
	}
	return count
}

func hasPrefixFold(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	for i := 0; i < len(prefix); i++ {
		sc := s[i]
		pc := prefix[i]
		if sc == pc {
			continue
		}
		if 'a' <= sc && sc <= 'z' {
			sc = sc - 'a' + 'A'
		}
		if 'a' <= pc && pc <= 'z' {
			pc = pc - 'a' + 'A'
		}
		if sc != pc {
			return false
		}
	}
	return true
}

func isTextKeyword(token string) bool {
	if token == "" {
		return false
	}
	lastDot := strings.LastIndexByte(token, '.')
	if lastDot >= 0 && lastDot+1 < len(token) {
		token = token[lastDot+1:]
	}
	return textKeywords[strings.ToUpper(token)]
}
