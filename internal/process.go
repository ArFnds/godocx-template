package internal

import (
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

type ReportOutput struct {
	Report Node
	Images Images
	Links  Links
	Htmls  Htmls
}

type ReportData map[string]any

func (rd ReportData) GetValue(key string) (VarValue, bool) {
	return getValueFrom(key, rd)
}

func (rd ReportData) GetArray(key string) ([]any, bool) {
	value, ok := rd[key]
	if ok && isSlice(value) {
		value := reflect.ValueOf(value)
		ret := make([]any, value.Len())
		for i := 0; i < value.Len(); i++ {
			element := value.Index(i)
			ret[i] = element.Interface()
		}
		return ret, true
	}
	return nil, false
}

func (rd ReportData) GetImage(key string) (*ImagePars, bool) {
	value, ok := rd[key]
	if ok {
		return value.(*ImagePars), true
	}
	return nil, false
}

type CommandProcessor func(data *ReportData, node Node, ctx *Context) (string, error)

var (
	IncompleteConditionalStatementError = errors.New("IncompleteConditionalStatementError")
	IgnoreError                         = errors.New("ignore")
	BUILT_IN_COMMANDS                   = []string{
		"CMD_NODE",
		"ALIAS",
		"FOR",
		"END-FOR",
		"IF",
		"END-IF",
		"INS",
		"IMAGE",
		"LINK",
		"HTML",
	}
)

func ProduceReport(data *ReportData, template Node, ctx Context) (*ReportOutput, error) {
	return walkTemplate(data, template, &ctx, processCmd)
}

func notBuiltIns(cmd string) bool {
	upperCmd := strings.ToUpper(cmd)
	return !slices.ContainsFunc(BUILT_IN_COMMANDS, func(b string) bool { return strings.HasPrefix(upperCmd, b) })
}

func getCommand(command string, shorthands map[string]string, fixSmartQuotes bool) (string, error) {

	cmd := strings.TrimSpace(command)
	runes := []rune(cmd)

	if runes[0] == '*' {
		aliasName := string(runes[1:])
		if foundCmd, ok := shorthands[aliasName]; ok {
			cmd = foundCmd
			slog.Debug("Alias for command", "command", cmd)
		} else {
			return "", errors.New("Unknown alias: " + aliasName)
		}
	} else if runes[0] == '=' {
		cmd = "INS " + string(runes[1:])
	} else if runes[0] == '!' {
		cmd = "EXEC " + string(runes[1:])
	} else if notBuiltIns(cmd) {
		cmd = "INS " + cmd
	}

	if fixSmartQuotes {
		replacer := strings.NewReplacer(
			"“", `"`, // \u201C
			"”", `"`, // \u201D
			"„", `"`, // \u201E
			"‘", "'", // \u2018
			"’", "'", // \u2019
			"‚", "'", // \u201A
		)
		cmd = replacer.Replace(cmd)
	}

	return strings.TrimSpace(cmd), nil
}

func splitCommand(cmd string) (cmdName string, rest string) {
	// const cmdNameMatch = /^(\S+)\s*/.exec(cmd);
	re := regexp.MustCompile(`^(\S+)\s*`)

	cmdNameMatch := re.FindStringSubmatch(cmd)

	if len(cmdNameMatch) > 0 {
		cmdName = strings.ToUpper(cmdNameMatch[1])
		rest = strings.TrimSpace(cmd[len(cmdName):])
		return
	}
	return
}

func processForIf(data *ReportData, node Node, ctx *Context, cmd string, cmdName string, cmdRest string) error {
	isIf := cmdName == "IF"

	var forMatch []string
	var varName string

	if isIf {
		if node.Name() == "" {
			node.SetName("__if_" + fmt.Sprint(ctx.gCntIf))
			ctx.gCntIf++
		}
		varName = node.Name()
	} else {
		re := regexp.MustCompile(`(?i)^(\S+)\s+IN\s+(.+)$`)
		forMatch = re.FindStringSubmatch(cmdRest)
		if forMatch == nil {
			return errors.New("Invalid FOR command")
		}
		varName = forMatch[1]
	}

	// Have we already seen this node or is it the start of a new FOR loop?
	curLoop := getCurLoop(ctx)
	if !(curLoop != nil && curLoop.varName == varName) {
		if isIf {
			// Check if there is already an IF statement with the same name
			for _, loop := range ctx.loops {
				if loop.isIf && loop.varName == varName {
					return NewInvalidCommandError("Duplicate IF statement", cmd)
				}
			}
		}

		parentLoopLevel := len(ctx.loops) - 1
		fParentIsExploring := parentLoopLevel >= 0 && ctx.loops[parentLoopLevel].idx == -1
		var loopOver []VarValue

		if fParentIsExploring {
			loopOver = []VarValue{}
		} else if isIf {
			// Evaluate IF condition expression
			shouldRun, err := runAndGetValue(cmdRest, ctx, data)
			if err != nil {
				return err
			}
			// Determine whether to execute the IF block based on the condition result
			if shouldRun != nil {
				// Handle all non-nil values uniformly
				switch v := shouldRun.(type) {
				case bool:
					if v {
						loopOver = []VarValue{1}
					} else {
						loopOver = []VarValue{}
					}
				case string:
					if v != "" {
						loopOver = []VarValue{1}
					} else {
						loopOver = []VarValue{}
					}
				default:
					// For numeric types and other types, treat as true if not nil
					loopOver = []VarValue{1}
				}
			} else {
				// nil value is treated as false
				loopOver = []VarValue{}
			}
		} else {
			if forMatch == nil {
				return errors.New("Invalid FOR command")
			}
			items, err := runAndGetValue(forMatch[2], ctx, data)
			if err != nil {
				return fmt.Errorf("Invalid FOR command (can only iterate over Array) %s: %w", forMatch[2], err)
			}
			reflected := reflect.ValueOf(items)
			if reflected.Kind() != reflect.Slice {
				return fmt.Errorf("Invalid FOR command (can only iterate over Array) %s: %v", forMatch[2], reflected.Kind())
			}
			for i := 0; i < reflected.Len(); i++ {
				item := reflected.Index(i).Interface()
				loopOver = append(loopOver, item)
			}
		}
		ctx.loops = append(ctx.loops, LoopStatus{
			refNode:      node,
			refNodeLevel: ctx.level,
			varName:      varName,
			loopOver:     loopOver,
			isIf:         isIf,
			idx:          -1,
		})
	}
	logLoop(ctx.loops)

	return nil
}

func processEndForIf(node Node, ctx *Context, cmd string, cmdName string, cmdRest string) error {
	isIf := cmdName == "END-IF"
	curLoop := getCurLoop(ctx)

	if curLoop == nil {
		contextType := "IF statement"
		if !isIf {
			contextType = "FOR loop"
		}
		errorMessage := fmt.Sprintf("Unexpected %s outside of %s context", cmdName, contextType)
		return NewInvalidCommandError(errorMessage, cmd)
	}

	// Ensure the current loop is an IF statement
	if isIf && !curLoop.isIf {
		return NewInvalidCommandError("END-IF found in FOR loop context", cmd)
	}

	// Reset the if check flag for the corresponding p or tr parent node
	parentPorTrNode := findParentPorTrNode(node)
	var parentPorTrNodeTag string
	if parentNodeNTxt, isParentNodeNTxt := parentPorTrNode.(*NonTextNode); isParentNodeNTxt {
		parentPorTrNodeTag = parentNodeNTxt.Tag
	}
	if parentPorTrNodeTag == P_TAG {
		delete(ctx.pIfCheckMap, node)
	} else if parentPorTrNodeTag == TR_TAG {
		delete(ctx.trIfCheckMap, node)
	}

	// First time we visit an END-IF node, we assign it the arbitrary name
	// generated when the IF was processed
	if isIf && node.Name() == "" {
		node.SetName(curLoop.varName)
		ctx.gCntEndIf += 1
	}

	// For END-IF, we don't need to check the variable name
	// For END-FOR, we still need to check the variable name
	varName := cmdRest
	if !isIf && curLoop.varName != varName {
		if !slices.ContainsFunc(ctx.loops, func(loop LoopStatus) bool { return loop.varName == varName }) {
			slog.Debug("Ignoring "+cmd+"("+varName+", but we're expecting "+curLoop.varName+")", "varName", varName)
			return nil
		}
		return NewInvalidCommandError("Invalid command", cmd)
	}

	// Get the next item in the loop
	nextIdx := curLoop.idx + 1
	var nextItem VarValue
	if nextIdx < len(curLoop.loopOver) {
		nextItem = curLoop.loopOver[nextIdx]
	}

	if nextItem != nil {
		// next iteration
		if !isIf {
			ctx.vars["$"+varName] = nextItem
			ctx.vars["$idx"] = nextIdx
		}
		ctx.fJump = true
		curLoop.idx = nextIdx
	} else {
		// loop finished
		// ctx.loops.pop()
		ctx.loops = ctx.loops[:len(ctx.loops)-1]
	}

	return nil
}

func validateImagePars(pars *ImagePars) error {
	err := validateExtension(pars.Extension)
	return err
}

func validateExtension(ext string) error {
	if !slices.Contains(ImageExtensions, ext) {
		return fmt.Errorf("An extension (one of %v) needs to be provided when providing an image or a thumbnail.", ImageExtensions)
	}
	return nil
}

func imageToContext(ctx *Context, img *Image) string {
	// TODO revalidate ? validateImage(img)
	ctx.imageAndShapeIdIncrement += 1
	id := fmt.Sprint(ctx.imageAndShapeIdIncrement)
	relId := fmt.Sprintf("img%s", id)
	ctx.images[relId] = img
	return relId
}

func getImageData(pars *ImagePars) *Image {
	return &Image{
		Extension: pars.Extension,
		Data:      pars.Data,
	}
}

func processImage(ctx *Context, imagePars *ImagePars) error {
	err := validateImagePars(imagePars)
	if err != nil {
		return err
	}

	cx := int(imagePars.Width * 360e3)
	cy := int(imagePars.Height * 360e3)

	imgRelId := imageToContext(ctx, getImageData(imagePars))
	id := fmt.Sprint(ctx.imageAndShapeIdIncrement)
	alt := imagePars.Alt
	if alt == "" {
		alt = "desc"
	}
	node := NewNonTextNode

	extNodes := make([]Node, 1)
	extNodes[0] = node("a:ext", map[string]string{
		"uri": "{28A0092B-C50C-407E-A947-70E740481C1C}",
	}, []Node{
		node("a14:useLocalDpi", map[string]string{
			"xmlns:a14": "http://schemas.microsoft.com/office/drawing/2010/main",
			"val":       "0",
		}, nil),
	})
	// http://officeopenxml.com/drwSp-rotate.php
	// Values are in 60,000ths of a degree, with positive angles moving clockwise or towards the positive y-axis.
	var rot string
	if imagePars.Rotation != 0 {
		rot = fmt.Sprintf("-%d", imagePars.Rotation*60e3)
	}

	if ctx.images[imgRelId].Extension == ".svg" {
		// Default to an empty thumbnail, as it is not critical and just part of the docx standard's scaffolding.
		// Without a thumbnail, the svg won't render (even in newer versions of Word that don't need the thumbnail).
		thumbnail := imagePars.Thumbnail
		if thumbnail == nil {
			thumbnail = &Thumbnail{
				Image: Image{Extension: ".png", Data: []byte{110, 111, 74, 68, 69, 110, 67, 10}},
			}
		}
		thumbRelId := imageToContext(ctx, &thumbnail.Image)
		extNodes = append(extNodes, node("a:ext", map[string]string{
			"uri": "{96DAC541-7B7A-43D3-8B79-37D633B846F1}",
		}, []Node{
			node("asvg:svgBlip", map[string]string{
				"xmlns:asvg": "http://schemas.microsoft.com/office/drawing/2016/SVG/main",
				"r:embed":    imgRelId,
			}, nil),
		}))
		// For SVG the thumb is placed where the image normally goes.
		imgRelId = thumbRelId
	}

	rotAttrs := map[string]string{}
	if rot != "" {
		rotAttrs["rot"] = rot
	}

	pic := node(
		"pic:pic",
		map[string]string{"xmlns:pic": "http://schemas.openxmlformats.org/drawingml/2006/picture"},
		[]Node{
			node("pic:nvPicPr", map[string]string{}, []Node{
				node("pic:cNvPr", map[string]string{"id": "0", "name": `Picture ` + id, "descr": alt}, nil),
				node("pic:cNvPicPr", map[string]string{}, []Node{
					node("a:picLocks", map[string]string{"noChangeAspect": "1", "noChangeArrowheads": "1"}, nil),
				}),
			}),
			node("pic:blipFill", map[string]string{}, []Node{
				node("a:blip", map[string]string{"r:embed": imgRelId, "cstate": "print"}, []Node{
					node("a:extLst", map[string]string{}, extNodes),
				}),
				node("a:srcRect", map[string]string{}, nil),
				node("a:stretch", map[string]string{}, []Node{node("a:fillRect", map[string]string{}, nil)}),
			}),
			node("pic:spPr", map[string]string{"bwMode": "auto"}, []Node{
				node("a:xfrm", rotAttrs, []Node{
					node("a:off", map[string]string{"x": "0", "y": "0"}, nil),
					node("a:ext", map[string]string{"cx": fmt.Sprint(cx), "cy": fmt.Sprint(cy)}, nil),
				}),
				node("a:prstGeom", map[string]string{"prst": "rect"}, []Node{node("a:avLst", map[string]string{}, nil)}),
				node("a:noFill", map[string]string{}, nil),
				node("a:ln", map[string]string{}, []Node{node("a:noFill", map[string]string{}, nil)}),
			}),
		},
	)
	drawing := node("w:drawing", map[string]string{}, []Node{
		node("wp:inline", map[string]string{"distT": "0", "distB": "0", "distL": "0", "distR": "0"}, []Node{
			node("wp:extent", map[string]string{"cx": fmt.Sprint(cx), "cy": fmt.Sprint(cy)}, nil),
			node("wp:docPr", map[string]string{"id": id, "name": `Picture ` + id, "descr": alt}, nil),
			node("wp:cNvGraphicFramePr", map[string]string{}, []Node{
				node("a:graphicFrameLocks", map[string]string{
					"xmlns:a":        "http://schemas.openxmlformats.org/drawingml/2006/main",
					"noChangeAspect": "1",
				}, nil),
			}),
			node(
				"a:graphic",
				map[string]string{"xmlns:a": "http://schemas.openxmlformats.org/drawingml/2006/main"},
				[]Node{
					node(
						"a:graphicData",
						map[string]string{"uri": "http://schemas.openxmlformats.org/drawingml/2006/picture"},
						[]Node{pic},
					),
				},
			),
		}),
	})

	ctx.pendingImageNode = &struct {
		image   *NonTextNode
		caption []*NonTextNode
	}{
		image:   drawing,
		caption: nil,
	}

	if imagePars.Caption != "" {
		ctx.pendingImageNode.caption = []*NonTextNode{
			node("w:br", map[string]string{}, nil),
			node("w:t", map[string]string{}, []Node{NewTextNode(imagePars.Caption)}),
		}
	}

	return nil
}
func processLink(ctx *Context, linkPars *LinkPars) error {
	url := linkPars.Url
	label := linkPars.Label
	if label == "" {
		label = url
	}

	ctx.linkId += 1
	id := fmt.Sprint(ctx.linkId)
	relId := "link" + id

	ctx.links[relId] = Link{
		url: url,
	}

	node := NewNonTextNode
	textRunPropsNode := ctx.textRunPropsNode

	if textRunPropsNode == nil {
		textRunPropsNode = node("w:rPr", nil, []Node{
			node("w:u", map[string]string{"w:val": "single"}, nil),
		})
	}

	link := node("w:hyperlink", map[string]string{"r:id": relId, "w:history": "1"}, []Node{
		node("w:r", nil, []Node{
			textRunPropsNode,
			node("w:t", nil, []Node{NewTextNode(label)}),
		}),
	})
	ctx.pendingLinkNode = link

	return nil
}

func findParentPorTrNode(node Node) (resultNode Node) {
	parentNode := node.Parent()

	for parentNode != nil && resultNode == nil {
		parentNTxtNode, isParentNTxtNode := parentNode.(*NonTextNode)
		var parentNodeTag string
		if isParentNTxtNode {
			parentNodeTag = parentNTxtNode.Tag
		}
		if parentNodeTag == P_TAG {
			var grandParentNode Node = nil
			if parentNode.Parent() != nil {
				grandParentNode = parentNode.Parent()
			}
			if grandParentNTxtNode, isGrandParentNTxtNode := grandParentNode.(*NonTextNode); grandParentNode != nil && isGrandParentNTxtNode && grandParentNTxtNode.Tag == TR_TAG {
				resultNode = grandParentNode
			} else {
				resultNode = parentNode
			}
		}
		parentNode = parentNode.Parent()
	}
	return
}

var functionCallRegexp = regexp.MustCompile(`(\w+)\(([^)]*)\)`)

func parseFunctionCall(rest string) ([]string, bool) {
	matches := functionCallRegexp.FindStringSubmatch(rest)
	if len(matches) > 0 {
		// parse args char by char to handle string containing commas
		args := []string{}
		current := strings.Builder{}
		isInString := false
		for _, char := range []rune(matches[2]) {
			if char == '\'' {
				current.WriteRune(char)
				isInString = !isInString
			} else if char == ',' && !isInString {
				args = append(args, strings.TrimSpace(current.String()))
				current.Reset()
			} else {
				current.WriteRune(char)
			}
		}
		args = append(args, strings.TrimSpace(current.String()))

		return append([]string{matches[1]}, args...), true
	}
	return nil, false
}

func isLink(varValue VarValue) (*LinkPars, bool) {
	if strMap, ok := varValue.(map[string]any); ok {
		url, hasUrl := strMap["url"].(string)
		label, _ := strMap["label"].(string)

		if hasUrl {
			return &LinkPars{
				Url:   url,
				Label: label,
			}, true
		}
	} else if linkPars, ok := varValue.(*LinkPars); ok {
		return linkPars, true
	}
	return nil, false
}

func getFromVars(ctx *Context, key string) (varValue VarValue, exists bool) {
	return getValueFrom(key, ctx.vars)
}

func getValue(key string, ctx *Context, data *ReportData) (VarValue, bool) {
	key = strings.TrimSpace(key)
	if key[0] == '$' {
		return getFromVars(ctx, key)
	}
	lastI := len(key) - 1
	if lastI > 0 && (key[0] == '\'' && key[lastI] == '\'') || (key[0] == '`' && key[lastI] == '`') {
		return key[1:lastI], true
	}
	if number, err := strconv.ParseInt(key, 10, 64); err == nil {
		return number, true
	}
	if number, err := strconv.ParseFloat(key, 64); err == nil {
		return number, true
	}
	return data.GetValue(key)
}

func runFunction(funcName string, args []string, ctx *Context, data *ReportData) (VarValue, error) {
	if ctx.options.Functions != nil {
		if function, ok := ctx.options.Functions[funcName]; ok {
			argValues := make([]any, len(args))
			for i, arg := range args {
				if varValue, ok := getValue(arg, ctx, data); ok {
					argValues[i] = varValue
				} else if ctx.options.ErrorHandler != nil {
					return ctx.options.ErrorHandler(&KeyNotFoundError{Key: arg}, arg), nil
				} else {
					return "", &KeyNotFoundError{Key: arg}
				}
			}
			value := function(argValues...)
			return value, nil
		} else {
			return "", &FunctionNotFoundError{FunctionName: funcName}
		}
	} else {
		return "", &FunctionNotFoundError{FunctionName: funcName}
	}
}

func runAndGetValue(text string, ctx *Context, data *ReportData) (VarValue, error) {
	var value VarValue
	// Process conditional expression
	// Check comparison operators
	for _, op := range []string{"==", "!=", ">=", "<=", ">", "<"} {
		if strings.Contains(text, op) {
			parts := strings.Split(text, op)
			if len(parts) == 2 {
				left, err := runAndGetValue(strings.TrimSpace(parts[0]), ctx, data)
				if err != nil {
					return nil, err
				}
				right, err := runAndGetValue(strings.TrimSpace(parts[1]), ctx, data)
				if err != nil {
					return nil, err
				}

				// Convert to numeric values for comparison
				leftNum, leftIsNum := toNumber(left)
				rightNum, rightIsNum := toNumber(right)

				if leftIsNum && rightIsNum {
					switch op {
					case "==":
						return leftNum == rightNum, nil
					case ">=":
						return leftNum >= rightNum, nil
					case "<=":
						return leftNum <= rightNum, nil
					case ">":
						return leftNum > rightNum, nil
					case "<":
						return leftNum < rightNum, nil
					}
				} else if op == "==" {
					// For non-numeric values,handle equality comparison
					return left == right, nil
				} else if op == "!=" {
					// For non-numeric values, handle not equality comparison
					return left != right, nil
				}
			}
		}
	}

	if args, isFunction := parseFunctionCall(text); isFunction {
		funcName := args[0]
		args = args[1:]

		var err error
		value, err = runFunction(funcName, args, ctx, data)
		if err != nil {
			return "", err
		}
	} else if varValue, ok := getValue(text, ctx, data); ok {
		return varValue, nil
	} else if ctx.options.ErrorHandler != nil {
		value = ctx.options.ErrorHandler(&KeyNotFoundError{Key: text}, text)
	} else {
		return "", errors.New("Fail to compute value for " + text)
	}
	return value, nil
}

// Helper function: Convert value to float64 for comparison
func toNumber(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case int:
		return float64(val), true
	case int32:
		return float64(val), true
	case int64:
		return float64(val), true
	case float32:
		return float64(val), true
	case float64:
		return val, true
	case string:
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

func processHtml(html string, ctx *Context) {
	interpolationRegex := regexp.MustCompile(`\$\{(.*?)\}`)

	html = interpolationRegex.ReplaceAllStringFunc(html, func(match string) string {
		key := match[2 : len(match)-1]
		value, _ := runAndGetValue(key, ctx, nil)
		return fmt.Sprint(value)
	})

	ctx.htmlId += 1
	id := fmt.Sprint(ctx.htmlId)
	relId := "html" + id
	ctx.htmls[relId] = html
	htmlNode := NewNonTextNode(ALTCHUNK_TAG, map[string]string{"r:id": relId}, nil)
	ctx.pendingHtmlNode = htmlNode
}

func processCmd(data *ReportData, node Node, ctx *Context) (string, error) {
	cmd, err := getCommand(ctx.cmd, ctx.shorthands, ctx.options.FixSmartQuotes)

	if err != nil {
		return "", err
	}
	ctx.cmd = "" // flush the context

	cmdName, rest := splitCommand(cmd)

	//if (cmdName !== "CMD_NODE") logger.debug(`Processing cmd: ${cmd}`);
	if cmdName != "CMD_NODE" {
		slog.Debug("Processing cmd", "cmd", cmd)
	}

	if ctx.fSeekQuery {
		if cmdName == "QUERY" {
			ctx.query = rest
		}
		return "", nil
	}

	if cmdName == "CMD_NODE" || rest == "CMD_NODE" {
		// logger.debug(`Ignoring ${cmdName} command`);
		return "", IgnoreError
		// ALIAS name ANYTHING ELSE THAT MIGHT BE PART OF THE COMMAND...
	} else if cmdName == "ALIAS" {
		aliasRegexp := regexp.MustCompile(`^(\S+)\s*(.*)`)
		aliasMatch := aliasRegexp.FindStringSubmatch(rest)
		if len(aliasMatch) == 3 {
			ctx.shorthands[aliasMatch[1]] = aliasMatch[2]
			slog.Debug("Defined alias", "alias", aliasMatch[1], "for", aliasMatch[2])
		}
		// FOR <varName> IN <expression>
		// IF <expression>
	} else if cmdName == "FOR" || cmdName == "IF" {
		err := processForIf(data, node, ctx, cmd, cmdName, rest)
		if err != nil {
			return "", err
		}

		// END-FOR
		// END-IF
	} else if cmdName == "END-FOR" || cmdName == "END-IF" {
		err := processEndForIf(node, ctx, cmd, cmdName, rest)
		if err != nil {
			return "", err
		}

		// INS <expression>
	} else if cmdName == "INS" {
		if !isLoopExploring(ctx) {

			varValue, err := runAndGetValue(rest, ctx, data)
			if err != nil {
				return "", err
			}
			value := fmt.Sprintf("%v", varValue)

			if ctx.options.ProcessLineBreaks {
				literalXmlDelimiter := ctx.options.LiteralXmlDelimiter
				if ctx.options.ProcessLineBreaksAsNewText {
					splitByLineBreak := strings.Split(value, "\n")
					LINE_BREAK := literalXmlDelimiter + `<w:br/>` + literalXmlDelimiter
					END_OF_TEXT := literalXmlDelimiter + `</w:t>` + literalXmlDelimiter
					START_OF_TEXT := literalXmlDelimiter + `<w:t xml:space="preserve">` + literalXmlDelimiter

					value = strings.Join(splitByLineBreak, LINE_BREAK+START_OF_TEXT+END_OF_TEXT)
				} else {
					value = strings.ReplaceAll(value, "\n", literalXmlDelimiter+"<w:br/>"+literalXmlDelimiter)
				}
			}

			return value, nil
		}
		// IMAGE <code>
	} else if cmdName == "IMAGE" {
		if !isLoopExploring(ctx) {
			varValue, err := runAndGetValue(rest, ctx, data)
			if err != nil {
				return "", err
			}

			if imgPars, ok := varValue.(*ImagePars); ok {
				err := processImage(ctx, imgPars)
				if err != nil {
					return "", fmt.Errorf("ImageError: %w", err)
				}
			} else {
				return "", errors.New("Not an image as result of " + rest)
			}
		}

		// LINK <code>
	} else if cmdName == "LINK" {
		if !isLoopExploring(ctx) {
			pars, err := runAndGetValue(rest, ctx, data)
			if err != nil {
				return "", fmt.Errorf("LinkError: %w", err)
			}
			if linkPars, ok := isLink(pars); ok {
				err := processLink(ctx, linkPars)
				if err != nil {
					return "", fmt.Errorf("LinkError: %w", err)
				}
			}
		}
	} else if cmdName == "HTML" {
		if !isLoopExploring(ctx) {
			varValue, err := runAndGetValue(rest, ctx, data)
			if err != nil {
				return "", err
			}
			processHtml(fmt.Sprintf("%v", varValue), ctx)
			return "", nil
		}

		// CommandSyntaxError
	} else {
		return "", errors.New("CommandSyntaxError: " + cmd)
	}

	return "", IgnoreError
}

func debugPrintNode(node Node) string {
	switch n := node.(type) {
	case *NonTextNode:
		return fmt.Sprintf("<%s> %v", n.Tag, n.Attrs)
	case *TextNode:
		return n.Text
	default:
		return "<unknown>"
	}
}
func walkTemplate(data *ReportData, template Node, ctx *Context, processor CommandProcessor) (*ReportOutput, error) {
	var retErr error
	out := CloneNodeWithoutChildren(template.(*NonTextNode))

	nodeIn := template
	var nodeOut Node = out
	move := ""
	deltaJump := 0

	loopCount := 0
	// TODO get from options
	maximumWalkingDepth := 1_000_000

	for {
		curLoop := getCurLoop(ctx)
		var nextSibling Node = nil

		// =============================================
		// Move input node pointer
		// =============================================
		if ctx.fJump {
			if curLoop == nil {
				return nil, errors.New("jumping while curLoop is nil")
			}
			slog.Debug("Jumping to level", "level", curLoop.refNodeLevel)
			deltaJump = ctx.level - curLoop.refNodeLevel
			nodeIn = curLoop.refNode
			ctx.level = curLoop.refNodeLevel
			ctx.fJump = false
			move = "JUMP"

			// Down (only if he haven't just moved up)
		} else if len(nodeIn.Children()) > 0 && move != "UP" {
			nodeIn = nodeIn.Children()[0]
			ctx.level += 1
			move = "DOWN"

			// Sideways
		} else if nextSibling = getNextSibling(nodeIn); nextSibling != nil {
			nodeIn = nextSibling
			move = "SIDE"

			// Up
		} else {
			parent := nodeIn.Parent()
			if parent == nil {
				slog.Debug("=== parent is null, breaking after %s loops...", "loopCount", loopCount)
				break
			} else if loopCount > maximumWalkingDepth {
				slog.Debug("=== parent is still not null after {loopCount} loops, something must be wrong ...", "loopCount", loopCount)
				return nil, errors.New("infinite loop or massive dataset detected. Please review and try again")
			}
			nodeIn = parent
			ctx.level -= 1
			move = "UP"
		}

		//slog.Debug(`Next node`, "move", move, "level", ctx.level, "nodeIn", debugPrintNode(nodeIn))

		// =============================================
		// Process input node
		// =============================================
		// Delete the last generated output node in several special cases
		// --------------------------------------------------------------
		if move != "DOWN" {
			nonTextNodeOut, isNodeOutNonText := nodeOut.(*NonTextNode)
			var tag string
			if isNodeOutNonText {
				tag = nonTextNodeOut.Tag
			}
			fRemoveNode := false

			// Delete last generated output node if we're skipping nodes due to an empty FOR loop
			if (tag == P_TAG ||
				tag == TBL_TAG ||
				tag == TR_TAG ||
				tag == TC_TAG) && isLoopExploring(ctx) {
				fRemoveNode = true
				// Delete last generated output node if the user inserted a paragraph
				// (or table row) with just a command
			} else if tag == P_TAG || tag == TR_TAG || tag == TC_TAG {
				buffers := ctx.buffers[tag]
				fRemoveNode = buffers.text == "" && buffers.cmds != "" && !buffers.fInsertedText
			}

			// Execute removal, if needed. The node will no longer be part of the output, but
			// the parent will be accessible from the child (so that we can still move up the tree)
			if fRemoveNode && nodeOut.Parent() != nil {
				nodeOut.Parent().PopChild()
			}

		}

		// Handle an UP movement
		// ---------------------
		if move == "UP" {
			// Loop exploring? Update the reference node for the current loop
			if isLoopExploring(ctx) && curLoop != nil && nodeIn == curLoop.refNode.Parent() {
				curLoop.refNode = nodeIn
				curLoop.refNodeLevel -= 1
			}
			nodeOutParent := nodeOut.Parent()
			if nodeOutParent == nil {
				return nil, errors.New("nodeOut has no parent")
			}
			// Execute the move in the output tree
			nodeOut = nodeOutParent

			nonTextNodeOut, isNotTextNode := nodeOut.(*NonTextNode)
			// If an image was generated, replace the parent `w:t` node with
			// the image node
			if isNotTextNode && ctx.pendingImageNode != nil && nonTextNodeOut.Tag == T_TAG {
				imgNode := ctx.pendingImageNode.image
				captionNodes := ctx.pendingImageNode.caption
				parent := nodeOut.Parent()
				if parent != nil {
					imgNode.SetParent(parent)
					// pop last children
					parent.PopChild()
					parent.AddChild(imgNode)
					if len(captionNodes) > 0 {
						for _, captionNode := range captionNodes {
							captionNode.SetParent(parent)
							parent.AddChild(captionNode)
						}
					}

					// Prevent containing paragraph or table row from being removed
					ctx.buffers[P_TAG].fInsertedText = true
					ctx.buffers[TR_TAG].fInsertedText = true
					ctx.buffers[TC_TAG].fInsertedText = true
				}
				ctx.pendingImageNode = nil
			}

			// If a link was generated, replace the parent `w:r` node with
			// the link node
			if ctx.pendingLinkNode != nil && isNotTextNode && nonTextNodeOut.Tag == R_TAG {
				linkNode := ctx.pendingLinkNode
				parent := nodeOut.Parent()
				if parent != nil {
					linkNode.SetParent(parent)
					// pop last children
					parent.PopChild()
					parent.AddChild(linkNode)
					// Prevent containing paragraph or table row from being removed
					ctx.buffers[P_TAG].fInsertedText = true
					ctx.buffers[TR_TAG].fInsertedText = true
					ctx.buffers[TC_TAG].fInsertedText = true
				}
				ctx.pendingLinkNode = nil
			}

			// If a html page was generated, replace the parent `w:p` node with
			// the html node
			if ctx.pendingHtmlNode != nil && isNotTextNode && nonTextNodeOut.Tag == P_TAG {
				htmlNode := ctx.pendingHtmlNode
				parent := nodeOut.Parent()
				if parent != nil {
					htmlNode.SetParent(parent)
					// pop last children
					parent.PopChild()
					parent.AddChild(htmlNode)
					// Prevent containing paragraph or table row from being removed
					ctx.buffers[P_TAG].fInsertedText = true
					ctx.buffers[TR_TAG].fInsertedText = true
					ctx.buffers[TC_TAG].fInsertedText = true
				}
				ctx.pendingHtmlNode = nil
			}

			// `w:tc` nodes shouldn't be left with no `w:p` or 'w:altChunk' children; if that's the
			// case, add an empty `w:p` inside
			filterCase := slices.ContainsFunc(nodeOut.Children(), func(node Node) bool {
				nonTextNode, isNotTextNode := node.(*NonTextNode)
				return isNotTextNode && (nonTextNode.Tag == P_TAG || nonTextNode.Tag == ALTCHUNK_TAG)
			})
			if isNotTextNode && nonTextNodeOut.Tag == TC_TAG && !filterCase {
				nodeOut.AddChild(NewNonTextNode(P_TAG, nil, nil))
			}

			// Save latest `w:rPr` node that was visited (for LINK properties)
			if isNotTextNode && nonTextNodeOut.Tag == RPR_TAG {
				ctx.textRunPropsNode = nonTextNodeOut
			}
			if isNotTextNode && nonTextNodeOut.Tag == R_TAG {
				ctx.textRunPropsNode = nil
			}
		}

		// Node creation: DOWN | SIDE
		// --------------------------
		// Note that nodes are copied to the new tree, but that doesn't mean they will be kept.
		// In some cases, they will be removed later on; for example, when a paragraph only
		// contained a command -- it will be deleted.
		if move == "DOWN" || move == "SIDE" {
			// Move nodeOut to point to the new node's parent
			if move == "SIDE" {
				if nodeOut.Parent() == nil {
					return nil, errors.New("Template syntax error: node has no parent")
				}
				nodeOut = nodeOut.Parent()
			}
			// Reset node buffers as needed if a `w:p` or `w:tr` is encountered
			nodeInNTxt, isNodeInNTxt := nodeIn.(*NonTextNode)
			var tag string
			if isNodeInNTxt {
				tag = nodeInNTxt.Tag
			}
			if tag == P_TAG || tag == TR_TAG || tag == TC_TAG {
				ctx.buffers[tag] = &BufferStatus{text: "", cmds: "", fInsertedText: false}
			}

			newNode := CloneNodeWithoutChildren(nodeIn)
			newNode.SetParent(nodeOut)
			nodeOut.AddChild(newNode)

			// Update shape IDs in mc:AlternateContent
			if isNodeInNTxt {
				newNodeTag := nodeInNTxt.Tag
				if !isLoopExploring(ctx) && (newNodeTag == DOCPR_TAG || newNodeTag == VSHAPE_TAG) {
					slog.Debug("detected a - ", "newNode", debugPrintNode(newNode))
					updateID(newNode.(*NonTextNode), ctx)
				}
			}

			// If it's a text node inside a w:t, process it
			parent := nodeIn.Parent()
			nodeInTxt, isNodeInTxt := nodeIn.(*TextNode)
			nodeInParentNTxt, isNodeInParentNTxt := parent.(*NonTextNode)
			if isNodeInTxt && parent != nil && isNodeInParentNTxt && nodeInParentNTxt.Tag == T_TAG {
				result, err := processText(data, nodeInTxt, ctx, processor)
				if err != nil {
					retErr = errors.Join(retErr, err)
				} else {
					newNode.(*TextNode).Text = result
					slog.Debug("Inserted command result string into node. Updated node: ", "node", debugPrintNode(newNode))
				}
			}
			// Execute the move in the output tree
			nodeOut = newNode
		}

		// JUMP to the target level of the tree.
		// -------------------------------------------
		if move == "JUMP" {
			for deltaJump > 0 {
				if nodeOut.Parent() == nil {
					return nil, errors.New("Template syntax error: node has no parent")
				}
				nodeOut = nodeOut.Parent()
				deltaJump--
			}
		}

		loopCount++
	}

	if ctx.gCntIf != ctx.gCntEndIf {
		if ctx.options.FailFast {
			return nil, IncompleteConditionalStatementError
		} else {

			retErr = errors.Join(retErr, IncompleteConditionalStatementError)
		}
	}

	hasOtherThanIf := slices.ContainsFunc(ctx.loops, func(loop LoopStatus) bool { return !loop.isIf })
	if hasOtherThanIf {
		innerMostLoop := ctx.loops[len(ctx.loops)-1]
		retErr = errors.Join(retErr, fmt.Errorf("Unterminated FOR-loop ('FOR %s", innerMostLoop.varName))
		if ctx.options.FailFast {
			return nil, retErr
		} else {
			retErr = errors.Join(retErr, IncompleteConditionalStatementError)
		}
	}

	return &ReportOutput{
		Report: out,
		Images: ctx.images,
		Links:  ctx.links,
		Htmls:  ctx.htmls,
	}, retErr

}

func processText(data *ReportData, node *TextNode, ctx *Context, onCommand CommandProcessor) (string, error) {
	cmdDelimiter := ctx.options.CmdDelimiter
	failFast := ctx.options.FailFast

	text := node.Text
	if text == "" {
		return "", nil
	}

	segments := splitTextByDelimiters(text, *cmdDelimiter)
	outText := ""
	errorsList := []error{}

	for idx, segment := range segments {
		if idx > 0 {
			// Include the separators in the `buffers` field (used for deleting paragraphs if appropriate)
			appendTextToTagBuffers(cmdDelimiter.Open, ctx, map[string]bool{"fCmd": true})
		}
		// Append segment either to the `ctx.cmd` buffer (to be executed), if we are in "command mode",
		// or to the output text
		if ctx.fCmd {
			ctx.cmd += segment
		} else if !isLoopExploring(ctx) {
			outText += segment
		}
		appendTextToTagBuffers(segment, ctx, map[string]bool{"fCmd": ctx.fCmd})

		// If there are more segments, execute the command (if we are in "command mode"),
		// and toggle "command mode"
		if idx < len(segments)-1 {
			if ctx.fCmd {
				cmdResultText, err := onCommand(data, node, ctx)
				if err != nil && err != IgnoreError {
					if failFast {
						return "", err
					} else {
						errorsList = append(errorsList, err)
					}
				} else if err != IgnoreError {
					outText += cmdResultText
					appendTextToTagBuffers(cmdResultText, ctx, map[string]bool{
						"fCmd":          false,
						"fInsertedText": true,
					})
				}
			}
			ctx.fCmd = !ctx.fCmd
		}
	}
	if len(errorsList) > 0 {
		return "", errors.Join(errorsList...)
	}
	return outText, nil
}

func splitTextByDelimiters(text string, delimiters Delimiters) []string {
	segments := strings.Split(text, delimiters.Open)
	var result []string
	for _, seg := range segments {
		result = append(result, strings.Split(seg, delimiters.Close)...)
	}
	return result
}

var BufferKeys []string = []string{P_TAG, TR_TAG, TC_TAG}

func appendTextToTagBuffers(text string, ctx *Context, options map[string]bool) {
	if ctx.fSeekQuery {
		return
	}

	fCmd := options["fCmd"]
	fInsertedText := options["fInsertedText"]
	typeKey := "text"
	if fCmd {
		typeKey = "cmds"
	}

	for _, key := range BufferKeys {
		buf := ctx.buffers[key]
		if typeKey == "cmds" {
			buf.cmds += text
		} else {
			buf.text += text
		}
		if fInsertedText {
			buf.fInsertedText = true
		}
	}
}

func formatErrors(errorsList []error) string {
	errMsgs := []string{}
	for _, err := range errorsList {
		errMsgs = append(errMsgs, err.Error())
	}
	return strings.Join(errMsgs, "; ")
}

func updateID(newNode *NonTextNode, ctx *Context) {
	ctx.imageAndShapeIdIncrement += 1
	id := fmt.Sprint(ctx.imageAndShapeIdIncrement)
	newNode.Attrs["id"] = id
}

func NewContext(options CreateReportOptions, imageAndShapeIdIncrement int) Context {
	builtin := map[string]Function{
		"len":  length,
		"join": join,
	}
	for k, v := range options.Functions {
		builtin[k] = v
	}
	options.Functions = builtin

	return Context{
		gCntIf:     0,
		gCntEndIf:  0,
		level:      1,
		fCmd:       false,
		cmd:        "",
		fSeekQuery: false,
		buffers: map[string]*BufferStatus{
			P_TAG:  {text: "", cmds: "", fInsertedText: false},
			TR_TAG: {text: "", cmds: "", fInsertedText: false},
			TC_TAG: {text: "", cmds: "", fInsertedText: false},
		},
		imageAndShapeIdIncrement: imageAndShapeIdIncrement,
		images:                   Images{},
		linkId:                   0,
		links:                    Links{},
		htmlId:                   0,
		htmls:                    Htmls{},
		vars:                     map[string]VarValue{},
		loops:                    []LoopStatus{},
		fJump:                    false,
		shorthands:               map[string]string{},
		options:                  options,
		// To verfiy we don't have a nested if within the same p or tr tag
		pIfCheckMap:  map[Node]string{},
		trIfCheckMap: map[Node]string{},
	}

}
