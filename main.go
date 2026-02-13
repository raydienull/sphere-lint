package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type lintIssue struct {
	file string
	line int
	kind string
	msg  string
}

type definitionLocation struct {
	file string
	line int
}

type referenceUse struct {
	file     string
	line     int
	defTypes []string
	id       string
}

type referencePattern struct {
	re       *regexp.Regexp
	defTypes []string
}

var (
	scriptsRoot      = "."
	scriptExtensions = []string{".scp"}
	ignoredDirs      = map[string]bool{
		".git":    true,
		"backups": true,
		"backup":  true,
		"trash":   true,
		".github": true,
	}

	defHeaderPattern     = regexp.MustCompile(`^\[(\w+)\s+([^\]]+)\]`)
	commentHeaderPattern = regexp.MustCompile(`(?i)^\[COMMENT(?:\s+[^\]]+)?\]`)
	triggerPattern       = regexp.MustCompile(`(?i)^\s*ON\s*=\s*@?.+`)

	refPatterns = []referencePattern{
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
		"TEMPLATE":   true,
	}

	textKeywords = map[string]bool{
		"SAY": true, "SYSMESSAGE": true, "MESSAGE": true, "EMOTE": true, "SAYU": true, "SAYUA": true,
		"TITLE": true, "NAME": true, "DESC": true, "PROMPTCONSOLE": true, "BARK": true, "GROUP": true,
		"EVENTS": true, "FLAGS": true, "RECT": true, "P": true, "AUTHOR": true, "PAGES": true,
	}

	bracketPairs = map[rune]rune{')': '(', ']': '[', '}': '{', '>': '<'}

	missingArgMessages = map[string]string{
		"WHILE":    "LOGIC: WHILE missing condition",
		"FOR":      "LOGIC: FOR missing expression",
		"DORAND":   "LOGIC: DORAND missing line count",
		"DOSWITCH": "LOGIC: DOSWITCH missing line number",
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

	itemAssignPattern      = regexp.MustCompile(`(?i)^\s*ITEM\s*=\s*(.*)$`)
	containerAssignPattern = regexp.MustCompile(`(?i)^\s*CONTAINER\s*=\s*(.*)$`)
	templateIdentPattern   = regexp.MustCompile(`(?i)\b[a-z_][a-z0-9_]*\b`)
)

func main() {
	defLocations := make(map[string]definitionLocation)
	defnameLocations := make(map[string]definitionLocation)
	idLocations := make(map[string]definitionLocation)
	var refUses []referenceUse
	var issues []lintIssue

	scannedFiles := 0
	filesWithIssues := make(map[string]bool)

	fmt.Println("=== SPHERE SCP LINT (Go Action) ===")

	err := filepath.WalkDir(scriptsRoot, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			issues = append(issues, lintIssue{file: path, line: 1, kind: "CRITICAL", msg: walkErr.Error()})
			return nil
		}
		if d.IsDir() {
			if ignoredDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !hasExtension(path, scriptExtensions) {
			return nil
		}
		scannedFiles++

		fileIssues := lintScriptFile(path, defLocations, defnameLocations, idLocations, &refUses)
		if len(fileIssues) > 0 {
			for _, issue := range fileIssues {
				filesWithIssues[issue.file] = true
			}
			issues = append(issues, fileIssues...)
		}
		return nil
	})
	if err != nil {
		issues = append(issues, lintIssue{file: scriptsRoot, line: 1, kind: "CRITICAL", msg: err.Error()})
	}

	undefinedIssues := findUndefinedReferences(refUses, defLocations, defnameLocations, idLocations)
	if len(undefinedIssues) > 0 {
		for _, issue := range undefinedIssues {
			filesWithIssues[issue.file] = true
		}
		issues = append(issues, undefinedIssues...)
	}

	for _, issue := range issues {
		printError(issue)
	}

	fmt.Println("---------------------------------------------")
	fmt.Printf("Files scanned: %d\n", scannedFiles)
	fmt.Printf("Files with errors: %d\n", len(filesWithIssues))
	fmt.Printf("Total errors: %d\n", len(issues))

	if len(issues) > 0 {
		os.Exit(1)
	}
}

func lintScriptFile(path string, defIndex map[string]definitionLocation, defnameIndex map[string]definitionLocation, idIndex map[string]definitionLocation, references *[]referenceUse) []lintIssue {
	var issues []lintIssue
	var stack []blockState
	inTextBlock := false
	currentSection := ""

	rel := toRelative(path)

	file, err := os.Open(path)
	if err != nil {
		return []lintIssue{{file: rel, line: 1, kind: "CRITICAL", msg: err.Error()}}
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
			issues = appendUnclosedStackErrors(issues, stack, rel, lineNum, " before new section.", false)
			inTextBlock = true
			stack = nil
			continue
		}

		if defMatch := defHeaderPattern.FindStringSubmatch(cleaned); len(defMatch) == 3 {
			issues = appendUnclosedStackErrors(issues, stack, rel, lineNum, " before new section.", false)
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
					recordIdentifier(idIndex, id, rel, lineNum)
					key := defType + " " + id
					if defType == "DIALOG" && len(fields) > 1 {
						subType := strings.ToUpper(fields[1])
						if subType == "TEXT" || subType == "BUTTON" {
							key = key + " " + subType
						}
					}
					if prev, ok := defIndex[key]; ok {
						issues = append(issues, lintIssue{
							file: rel,
							line: lineNum,
							kind: "DUPLICATE",
							msg:  fmt.Sprintf("DUPLICATE: '%s' already defined at %s:%d.", key, prev.file, prev.line),
						})
					} else {
						defIndex[key] = definitionLocation{file: rel, line: lineNum}
					}
				}
			}
			stack = nil
			continue
		}

		if triggerPattern.MatchString(cleaned) {
			inTextBlock = false
			currentSection = ""
			issues = appendUnclosedStackErrors(issues, stack, rel, lineNum, " before new trigger.", false)
			stack = nil
			continue
		}

		if inTextBlock {
			continue
		}

		if isDefnameSection(currentSection) {
			fields := strings.Fields(cleaned)
			if len(fields) > 0 {
				recordDefName(defnameIndex, fields[0], rel, lineNum)
			}
		}

		if name := parseDefnameAssignment(cleaned); name != "" {
			upperName := strings.ToUpper(name)
			recordDefName(defnameIndex, upperName, rel, lineNum)
			if currentSection == "ITEMDEF" || currentSection == "CHARDEF" || currentSection == "TEMPLATE" {
				key := currentSection + " " + upperName
				if _, ok := defIndex[key]; !ok {
					defIndex[key] = definitionLocation{file: rel, line: lineNum}
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
				issues = appendError(issues, rel, lineNum, "SYNTAX", "SYNTAX: brackets -> "+bracketErr)
			}
		}

		if !isTextLine && !isAssignment {
			if upperToken == "DORAN" {
				issues = appendError(issues, rel, lineNum, "TYPO", "TYPO: 'DORAN' found. Did you mean 'DORAND'?")
			}
			if upperToken == "EN" {
				issues = appendError(issues, rel, lineNum, "TYPO", "TYPO: 'EN' found. Did you mean 'ENDO', 'ENDDO', or 'ENDIF'?")
			}
			if upperToken == "IF" && strings.TrimSpace(cleaned) == "IF" {
				issues = appendError(issues, rel, lineNum, "LOGIC", "LOGIC: empty 'IF' statement.")
			}
			if upperToken == "ELSEIF" && strings.TrimSpace(cleaned) == "ELSEIF" {
				issues = appendError(issues, rel, lineNum, "LOGIC", "LOGIC: empty 'ELSEIF' statement.")
			}
			if upperToken == "ELIF" && strings.TrimSpace(cleaned) == "ELIF" {
				issues = appendError(issues, rel, lineNum, "LOGIC", "LOGIC: empty 'ELIF' statement.")
			}
			trimmed := strings.TrimSpace(cleaned)
			if strings.HasPrefix(trimmed, "[EOF]") && trimmed != "[EOF]" {
				issues = appendError(issues, rel, lineNum, "CRITICAL", "CRITICAL: text found after [EOF].")
			}

			if upperToken != "" {
				fieldCount := countFields(cleaned, 2)
				if fieldCount < 2 {
					if msg, ok := missingArgMessages[upperToken]; ok {
						issues = appendError(issues, rel, lineNum, "LOGIC", msg)
					} else if forArgTokens[upperToken] {
						issues = appendError(issues, rel, lineNum, "LOGIC", fmt.Sprintf("LOGIC: %s missing argument", upperToken))
					}
				}

				if endToken := normalizeEndToken(upperToken); endToken != "" {
					if len(stack) == 0 {
						issues = appendError(issues, rel, lineNum, "BLOCK", fmt.Sprintf("BLOCK: '%s' without opening block.", upperToken))
					} else {
						last := stack[len(stack)-1]
						stack = stack[:len(stack)-1]
						expected := blockStartToEnd[last.typ]
						if endToken != expected {
							issues = appendError(issues, rel, lineNum, "BLOCK", fmt.Sprintf("BLOCK: mismatch. '%s' closed by '%s' (expected %s).", last.typ, upperToken, expected))
						}
					}
					continue
				}

				if upperToken == "ELSE" || upperToken == "ELIF" || upperToken == "ELSEIF" {
					if len(stack) == 0 || stack[len(stack)-1].typ != "IF" {
						issues = appendError(issues, rel, lineNum, "BLOCK", fmt.Sprintf("BLOCK: '%s' without matching IF.", upperToken))
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
			if currentSection == "TEMPLATE" {
				issues = append(issues, validateTemplateLine(cleaned, rel, lineNum)...)
				collectTemplateReferences(cleaned, rel, lineNum, references)
			}
			if !isAliasSection(currentSection) {
				collectReferenceUses(cleaned, rel, lineNum, references)
			}
		}
	}

	if scanErr := scanner.Err(); scanErr != nil {
		issues = appendError(issues, rel, lineNum, "CRITICAL", scanErr.Error())
	}

	if strings.ToUpper(strings.TrimSpace(lastNonEmpty)) != "[EOF]" {
		if lineNum == 0 {
			lineNum = 1
		}
		issues = appendError(issues, rel, lineNum, "CRITICAL", "CRITICAL: missing [EOF] at end of file.")
	}

	if len(stack) > 0 {
		issues = appendUnclosedStackErrors(issues, stack, rel, lineNum, ".", true)
	}

	return issues
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
				end, ok := scanAngleExpression(line, i+1)
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

func scanAngleExpression(line string, start int) (int, bool) {
	isEval := isAngleEvalStart(line, start)
	depth := 1
	parenDepth := 0
	for i := start; i < len(line); i++ {
		switch line[i] {
		case '<':
			if i+1 < len(line) && isAngleTokenStart(line[i+1]) {
				depth++
			}
		case '>':
			if depth > 1 {
				depth--
				if depth == 0 {
					return i, true
				}
				continue
			}
			if !isEval {
				depth--
				if depth == 0 {
					return i, true
				}
				continue
			}
			if isAngleCloseCandidate(line, i, parenDepth) {
				depth--
				if depth == 0 {
					return i, true
				}
			}
		default:
			if isEval {
				switch line[i] {
				case '(':
					parenDepth++
				case ')':
					if parenDepth > 0 {
						parenDepth--
					}
				}
			}
			if !isAngleTokenChar(line[i]) {
				continue
			}
		}
	}
	if isEval {
		end := len(line) - 1
		if end < 0 {
			end = 0
		}
		return end, true
	}
	return len(line), false
}

func isAngleCloseCandidate(line string, index int, parenDepth int) bool {
	if parenDepth > 0 {
		return false
	}
	if index+1 < len(line) && line[index+1] == '=' {
		return false
	}
	for i := index + 1; i < len(line); i++ {
		if line[i] == ' ' || line[i] == '\t' {
			continue
		}
		switch line[i] {
		case ')', ']', '}', ',', ';':
			return true
		default:
			return false
		}
	}
	return true
}

func isAngleEvalStart(line string, start int) bool {
	if start+4 > len(line) {
		return false
	}
	if strings.ToUpper(line[start:start+4]) != "EVAL" {
		return false
	}
	if start+4 >= len(line) {
		return true
	}
	next := line[start+4]
	return next == ' ' || next == '(' || next == '\t'
}

func isAngleTokenStart(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || b == '_'
}

func isAngleTokenChar(b byte) bool {
	return isAngleTokenStart(b) || (b >= '0' && b <= '9') || b == '.'
}

func printError(e lintIssue) {
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
	rel, err := filepath.Rel(scriptsRoot, path)
	if err != nil {
		return path
	}
	if rel == "." {
		return path
	}
	return filepath.ToSlash(rel)
}

func appendUnclosedStackErrors(errors []lintIssue, stack []blockState, rel string, lineNum int, msgSuffix string, useBlockLine bool) []lintIssue {
	if len(stack) == 0 {
		return errors
	}
	for _, b := range stack {
		errLine := lineNum
		if useBlockLine {
			errLine = b.line
		}
		errors = append(errors, lintIssue{file: rel, line: errLine, kind: "BLOCK", msg: fmt.Sprintf("BLOCK: unclosed '%s' block%s", b.typ, msgSuffix)})
	}
	return errors
}

func appendError(errors []lintIssue, rel string, lineNum int, kind, msg string) []lintIssue {
	return append(errors, lintIssue{file: rel, line: lineNum, kind: kind, msg: msg})
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

func collectReferenceUses(line, file string, lineNum int, references *[]referenceUse) {
	for _, pattern := range refPatterns {
		indices := pattern.re.FindAllStringIndex(line, -1)
		for _, idx := range indices {
			match := line[idx[0]:idx[1]]
			if shouldSkipDynamicID(line, idx[1], match) {
				continue
			}
			*references = append(*references, referenceUse{
				file:     file,
				line:     lineNum,
				defTypes: pattern.defTypes,
				id:       strings.ToUpper(match),
			})
		}
	}
}

func collectTemplateReferences(line, file string, lineNum int, references *[]referenceUse) {
	if match := itemAssignPattern.FindStringSubmatch(line); len(match) == 2 {
		for _, ident := range extractTemplateIdentifiers(match[1]) {
			*references = append(*references, referenceUse{
				file:     file,
				line:     lineNum,
				defTypes: []string{"ITEMDEF", "TEMPLATE"},
				id:       strings.ToUpper(ident),
			})
		}
		return
	}
	if match := containerAssignPattern.FindStringSubmatch(line); len(match) == 2 {
		for _, ident := range extractTemplateIdentifiers(match[1]) {
			*references = append(*references, referenceUse{
				file:     file,
				line:     lineNum,
				defTypes: []string{"ITEMDEF"},
				id:       strings.ToUpper(ident),
			})
		}
	}
}

func validateTemplateLine(line, file string, lineNum int) []lintIssue {
	var issues []lintIssue
	if match := itemAssignPattern.FindStringSubmatch(line); len(match) == 2 {
		value := strings.TrimSpace(match[1])
		if value == "" {
			issues = appendError(issues, file, lineNum, "LOGIC", "LOGIC: ITEM missing value")
			return issues
		}
		issues = appendTemplateSelectorIssues(issues, file, lineNum, value)
		return issues
	}
	if match := containerAssignPattern.FindStringSubmatch(line); len(match) == 2 {
		value := strings.TrimSpace(match[1])
		if value == "" {
			issues = appendError(issues, file, lineNum, "LOGIC", "LOGIC: CONTAINER missing value")
			return issues
		}
		issues = appendTemplateSelectorIssues(issues, file, lineNum, value)
	}
	return issues
}

func appendTemplateSelectorIssues(issues []lintIssue, file string, lineNum int, value string) []lintIssue {
	for _, msg := range validateTemplateRanges(value) {
		issues = appendError(issues, file, lineNum, "SYNTAX", msg)
	}
	for _, msg := range validateTemplateRSelectors(value) {
		issues = appendError(issues, file, lineNum, "SYNTAX", msg)
	}
	return issues
}

func validateTemplateRanges(value string) []string {
	var errors []string
	var stack []int
	for i := 0; i < len(value); i++ {
		switch value[i] {
		case '{':
			stack = append(stack, i)
		case '}':
			if len(stack) == 0 {
				continue
			}
			start := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if start+1 >= i {
				continue
			}
			segment := value[start+1 : i]
			parts := strings.Fields(segment)
			if len(parts) == 0 {
				continue
			}
			allNumeric := true
			for _, part := range parts {
				if !isAllDigits(part) {
					allNumeric = false
					break
				}
			}
			if !allNumeric {
				continue
			}
			if len(parts) != 2 {
				errors = append(errors, "SYNTAX: template range selector")
				continue
			}
			if isTemplateSpace(value[start+1]) || isTemplateSpace(value[i-1]) {
				errors = append(errors, "SYNTAX: template range selector")
			}
		}
	}
	return errors
}

func validateTemplateRSelectors(value string) []string {
	var errors []string
	tokens := templateIdentPattern.FindAllString(value, -1)
	for _, token := range tokens {
		if !isRSelectorCandidate(token) {
			continue
		}
		if !isAllDigits(token[1:]) {
			errors = append(errors, "SYNTAX: template R selector")
		}
	}
	return errors
}

func isRSelectorCandidate(token string) bool {
	if len(token) < 2 {
		return false
	}
	first := token[0]
	if first != 'R' && first != 'r' {
		return false
	}
	return token[1] >= '0' && token[1] <= '9'
}

func isTemplateSpace(b byte) bool {
	return b == ' ' || b == '\t'
}

func isAllDigits(value string) bool {
	if value == "" {
		return false
	}
	for i := 0; i < len(value); i++ {
		if value[i] < '0' || value[i] > '9' {
			return false
		}
	}
	return true
}

func extractTemplateIdentifiers(value string) []string {
	if value == "" {
		return nil
	}
	idents := templateIdentPattern.FindAllString(value, -1)
	if len(idents) == 0 {
		return nil
	}
	result := make([]string, 0, len(idents))
	for _, ident := range idents {
		if isTemplateSelectorToken(ident) {
			continue
		}
		result = append(result, ident)
	}
	return result
}

func isTemplateSelectorToken(token string) bool {
	upper := strings.ToUpper(token)
	if upper == "ITEM" || upper == "CONTAINER" {
		return true
	}
	if len(upper) > 1 && upper[0] == 'R' {
		for i := 1; i < len(upper); i++ {
			if upper[i] < '0' || upper[i] > '9' {
				return false
			}
		}
		return true
	}
	return false
}

func shouldSkipDynamicID(line string, end int, match string) bool {
	if end >= len(line) || line[end] != '<' {
		return false
	}
	if len(match) == 0 || match[len(match)-1] != '_' {
		return false
	}
	return true
}

func findUndefinedReferences(references []referenceUse, defIndex map[string]definitionLocation, defnameIndex map[string]definitionLocation, idIndex map[string]definitionLocation) []lintIssue {
	if len(references) == 0 {
		return nil
	}
	var errors []lintIssue
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
		errors = append(errors, lintIssue{
			file: ref.file,
			line: ref.line,
			kind: "UNDECLARED",
			msg:  fmt.Sprintf("UNDECLARED: '%s' not defined as %s", ref.id, typeLabel),
		})
	}
	return errors
}

func parseDefnameAssignment(line string) string {
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

func recordDefName(defnameIndex map[string]definitionLocation, name, file string, lineNum int) {
	upper := strings.ToUpper(name)
	if upper == "" {
		return
	}
	if _, ok := defnameIndex[upper]; ok {
		return
	}
	defnameIndex[upper] = definitionLocation{file: file, line: lineNum}
}

func recordIdentifier(idIndex map[string]definitionLocation, name, file string, lineNum int) {
	upper := strings.ToUpper(name)
	if upper == "" {
		return
	}
	if _, ok := idIndex[upper]; ok {
		return
	}
	idIndex[upper] = definitionLocation{file: file, line: lineNum}
}

func isAliasSection(section string) bool {
	return section == "RESDEFNAME" || section == "RES_RESDEFNAME"
}

func isDefnameSection(section string) bool {
	return section == "DEFNAME" || isAliasSection(section)
}
