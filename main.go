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

type idReference struct {
	file     string
	line     int
	defTypes []string
	id       string
}

type refPattern struct {
	re       *regexp.Regexp
	defTypes []string
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

	refPatterns = []refPattern{
		{re: regexp.MustCompile(`(?i)\bi_[a-z0-9_]+\b`), defTypes: []string{"ITEMDEF"}},
		{re: regexp.MustCompile(`(?i)\bc_[a-z0-9_]+\b`), defTypes: []string{"CHARDEF"}},
		{re: regexp.MustCompile(`(?i)\bspawn_[a-z0-9_]+\b`), defTypes: []string{"SPAWN"}},
		{re: regexp.MustCompile(`(?i)\bt_[a-z0-9_]+\b`), defTypes: []string{"TYPEDEF"}},
		{re: regexp.MustCompile(`(?i)\bs_[a-z0-9_]+\b`), defTypes: []string{"SPELL"}},
		{re: regexp.MustCompile(`(?i)\br_[a-z0-9_]+\b`), defTypes: []string{"REGIONTYPE", "AREADEF"}},
		{re: regexp.MustCompile(`(?i)\be_[a-z0-9_]+\b`), defTypes: []string{"EVENTS"}},
		{re: regexp.MustCompile(`(?i)\bm_[a-z0-9_]+\b`), defTypes: []string{"MENU"}},
		{re: regexp.MustCompile(`(?i)\bd_[a-z0-9_]+\b`), defTypes: []string{"DIALOG"}},
		{re: regexp.MustCompile(`(?i)\bf_[a-z0-9_]+\b`), defTypes: []string{"FUNCTION"}},
	}

	trackDefTypes = map[string]bool{
		"ITEMDEF":    true,
		"CHARDEF":    true,
		"EVENTS":     true,
		"FUNCTION":   true,
		"REGIONTYPE": true,
		"AREADEF":    true,
		"DIALOG":     true,
		"MENU":       true,
		"ROOMDEF":    true,
		"SKILL":      true,
		"SKILLCLASS": true,
		"SKILLMENU":  true,
		"SPAWN":      true,
		"SPELL":      true,
		"TYPEDEF":    true,
	}

	textKeywords = map[string]bool{
		"SAY": true, "SYSMESSAGE": true, "MESSAGE": true, "EMOTE": true, "SAYU": true, "SAYUA": true,
		"TITLE": true, "NAME": true, "DESC": true, "PROMPTCONSOLE": true, "BARK": true, "GROUP": true,
		"EVENTS": true, "FLAGS": true, "RECT": true, "P": true, "AUTHOR": true, "PAGES": true,
	}

	bracketPairs = map[rune]rune{')': '(', ']': '[', '}': '{', '>': '<'}

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
	defIndex := make(map[string]defLocation)
	defnameIndex := make(map[string]defLocation)
	idIndex := make(map[string]defLocation)
	var references []idReference
	var allErrors []lintError

	totalFiles := 0
	filesWithErrorsSet := make(map[string]bool)

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

		fileErrors := lintSCPFile(path, defIndex, defnameIndex, idIndex, &references)
		if len(fileErrors) > 0 {
			for _, e := range fileErrors {
				filesWithErrorsSet[e.file] = true
			}
			allErrors = append(allErrors, fileErrors...)
		}
		return nil
	})
	if err != nil {
		allErrors = append(allErrors, lintError{file: scriptsDir, line: 1, kind: "CRITICAL", msg: err.Error()})
	}

	undefErrors := validateUndefinedReferences(references, defIndex, defnameIndex, idIndex)
	if len(undefErrors) > 0 {
		for _, e := range undefErrors {
			filesWithErrorsSet[e.file] = true
		}
		allErrors = append(allErrors, undefErrors...)
	}

	for _, e := range allErrors {
		printError(e)
	}

	fmt.Println("---------------------------------------------")
	fmt.Printf("Files scanned: %d\n", totalFiles)
	fmt.Printf("Files with errors: %d\n", len(filesWithErrorsSet))
	fmt.Printf("Total errors: %d\n", len(allErrors))

	if len(allErrors) > 0 {
		os.Exit(1)
	}
}

func lintSCPFile(path string, defIndex map[string]defLocation, defnameIndex map[string]defLocation, idIndex map[string]defLocation, references *[]idReference) []lintError {
	var errors []lintError
	var stack []blockState
	inTextBlock := false
	currentSection := ""

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
			currentSection = defType
			if defType == "BOOK" || defType == "COMMENT" {
				inTextBlock = true
			} else {
				inTextBlock = false
			}
			if trackDefTypes[defType] {
				fields := strings.Fields(defArgs)
				id := ""
				if len(fields) > 0 {
					id = strings.ToUpper(fields[0])
				}
				if id != "" {
					recordID(idIndex, id, rel, lineNum)
					key := defType + " " + id
					if defType == "DIALOG" && len(fields) > 1 {
						subType := strings.ToUpper(fields[1])
						if subType == "TEXT" || subType == "BUTTON" {
							key = key + " " + subType
						}
					}
					if prev, ok := defIndex[key]; ok {
						errors = append(errors, lintError{
							file: rel,
							line: lineNum,
							kind: "DUPLICATE",
							msg:  fmt.Sprintf("DUPLICATE: '%s' already defined at %s:%d.", key, prev.file, prev.line),
						})
					} else {
						defIndex[key] = defLocation{file: rel, line: lineNum}
					}
				}
			}
			stack = nil
			continue
		}

		if triggerPattern.MatchString(cleaned) {
			inTextBlock = false
			currentSection = ""
			errors = appendUnclosedStackErrors(errors, stack, rel, lineNum, " before new trigger.", false)
			stack = nil
			continue
		}

		if inTextBlock {
			continue
		}

		if currentSection == "DEFNAME" {
			fields := strings.Fields(cleaned)
			if len(fields) > 0 {
				recordDefname(defnameIndex, fields[0], rel, lineNum)
			}
		}

		if currentSection == "ITEMDEF" || currentSection == "CHARDEF" {
			if name := parseDefnameValue(cleaned); name != "" {
				upperName := strings.ToUpper(name)
				recordDefname(defnameIndex, upperName, rel, lineNum)
				key := currentSection + " " + upperName
				if _, ok := defIndex[key]; !ok {
					defIndex[key] = defLocation{file: rel, line: lineNum}
				}
			}
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

		if !isTextLine && !isWriteFile {
			collectIDReferences(cleaned, rel, lineNum, references)
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
	for i := 0; i < len(line); i++ {
		ch := line[i]
		switch ch {
		case '(', '[', '{':
			stack = append(stack, rune(ch))
		case '<':
			if i+1 < len(line) && isAngleTokenStart(line[i+1]) {
				end, ok := scanAngleToken(line, i+1)
				if !ok {
					return "unclosed '<'"
				}
				i = end
				continue
			}
			continue
		case ')', ']', '}':
			if len(stack) == 0 {
				return fmt.Sprintf("unexpected closing '%c'", ch)
			}
			expected := bracketPairs[rune(ch)]
			if stack[len(stack)-1] != expected {
				return fmt.Sprintf("expected closing '%c' but found '%c'", stack[len(stack)-1], ch)
			}
			stack = stack[:len(stack)-1]
		case '>':
			continue
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

func scanAngleToken(line string, start int) (int, bool) {
	for i := start; i < len(line); i++ {
		ch := line[i]
		if !isAngleTokenChar(ch) {
			if ch == '>' {
				return i, true
			}
			return i, false
		}
	}
	return len(line), false
}

func isAngleTokenStart(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || b == '_'
}

func isAngleTokenChar(b byte) bool {
	return isAngleTokenStart(b) || (b >= '0' && b <= '9') || b == '.'
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

func collectIDReferences(line, file string, lineNum int, references *[]idReference) {
	for _, pattern := range refPatterns {
		matches := pattern.re.FindAllString(line, -1)
		for _, match := range matches {
			*references = append(*references, idReference{
				file:     file,
				line:     lineNum,
				defTypes: pattern.defTypes,
				id:       strings.ToUpper(match),
			})
		}
	}
}

func validateUndefinedReferences(references []idReference, defIndex map[string]defLocation, defnameIndex map[string]defLocation, idIndex map[string]defLocation) []lintError {
	if len(references) == 0 {
		return nil
	}
	var errors []lintError
	seen := make(map[string]bool)
	for _, ref := range references {
		if _, ok := defnameIndex[ref.id]; ok {
			continue
		}
		if _, ok := idIndex[ref.id]; ok {
			continue
		}
		found := false
		for _, defType := range ref.defTypes {
			key := defType + " " + ref.id
			if _, ok := defIndex[key]; ok {
				found = true
				break
			}
		}
		if found {
			continue
		}
		typeLabel := strings.Join(ref.defTypes, "/")
		errKey := ref.file + ":" + fmt.Sprintf("%d", ref.line) + ":" + ref.id + ":" + typeLabel
		if seen[errKey] {
			continue
		}
		seen[errKey] = true
		errors = append(errors, lintError{
			file: ref.file,
			line: ref.line,
			kind: "UNDECLARED",
			msg:  fmt.Sprintf("UNDECLARED: '%s' not defined as %s or DEFNAME.", ref.id, typeLabel),
		})
	}
	return errors
}

func parseDefnameValue(line string) string {
	if !hasPrefixFold(line, "DEFNAME=") {
		return ""
	}
	value := strings.TrimSpace(line[len("DEFNAME="):])
	if value == "" {
		return ""
	}
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func recordDefname(defnameIndex map[string]defLocation, name, file string, lineNum int) {
	upper := strings.ToUpper(name)
	if upper == "" {
		return
	}
	if _, ok := defnameIndex[upper]; ok {
		return
	}
	defnameIndex[upper] = defLocation{file: file, line: lineNum}
}

func recordID(idIndex map[string]defLocation, name, file string, lineNum int) {
	upper := strings.ToUpper(name)
	if upper == "" {
		return
	}
	if _, ok := idIndex[upper]; ok {
		return
	}
	idIndex[upper] = defLocation{file: file, line: lineNum}
}
