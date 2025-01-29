package internal

import (
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"regexp"
	"slices"
	"strings"
)

type ReportOutput struct {
	Report Node
	Images Images
	Links  Links
	Htmls  Htmls
}

type ReportData map[string]any

func (rd ReportData) GetValue(key string) (value VarValue, ok bool) {
	value, ok = rd[key]
	return
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

type CommandProcessor func(data ReportData, node Node, ctx *Context) (string, error)

var (
	IncompleteConditionalStatementError = errors.New("IncompleteConditionalStatementError")
	BUILT_IN_COMMANDS                   = []string{
		"QUERY",
		"CMD_NODE",
		"ALIAS",
		"FOR",
		"END-FOR",
		"IF",
		"END-IF",
		"INS",
		"EXEC",
		"IMAGE",
		"LINK",
		"HTML",
	}
)

func ProduceReport(data ReportData, template Node, ctx Context) (*ReportOutput, error) {
	return walkTemplate(data, template, &ctx, processCmd)
}

func notBuiltIns(cmd string) bool {
	upperCmd := strings.ToUpper(cmd)
	return !slices.ContainsFunc(BUILT_IN_COMMANDS, func(b string) bool { return strings.HasPrefix(upperCmd, b) })
}

func getCommand(command string, shorthands map[string]string, fixSmartQuotes bool) string {

	cmd := strings.TrimSpace(command)
	runes := []rune(cmd)

	if runes[0] == '*' {
		// TODO handle shorthands
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

	return strings.TrimSpace(cmd)
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

func processForIf(data ReportData, node Node, ctx *Context, cmd string, cmdName string, cmdRest string) error {
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

		}

		parentLoopLevel := len(ctx.loops) - 1
		fParentIsExploring := parentLoopLevel >= 0 && ctx.loops[parentLoopLevel].idx == -1
		var loopOver []VarValue

		if fParentIsExploring {
			loopOver = []VarValue{}
		} else if isIf {
			// TODO handle if
			//shouldRun, err := runUserJsAndGetRaw(data, cmdRest, ctx)
			//if err != nil {
			//	return err
			//}
			//if shouldRun {
			//	loopOver = []interface{}{1}
			//} else {
			//	loopOver = []interface{}{}
			//}
		} else {
			if forMatch == nil {
				return errors.New("Invalid FOR command")
			}
			items, ok := data.GetArray(forMatch[2])
			if !ok {
				return errors.New("Invalid FOR command (can only iterate over Array) " + cmd)
			}
			for _, item := range items {
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

	// Check if this is the expected END-IF/END-FOR. If not:
	// - If it's one of the nested varNames, throw
	// - If it's not one of the nested varNames, ignore it; we find
	//   cases in which an END-IF/FOR is found that belongs to a previous
	//   part of the paragraph of the current loop.
	varName := cmdRest
	if isIf {
		varName = node.Name()
	}
	if curLoop.varName != varName {
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
		ctx.vars["$"+varName] = nextItem
		ctx.fJump = true
		curLoop.idx = nextIdx
	} else {
		// loop finished
		// ctx.loops.pop()
		ctx.loops = ctx.loops[:len(ctx.loops)-1]
	}

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

func processCmd(data ReportData, node Node, ctx *Context) (string, error) {
	cmd := getCommand(ctx.cmd, ctx.shorthands, ctx.options.FixSmartQuotes)
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

	if cmdName == "QUERY" || cmdName == "CMD_NODE" {
		// logger.debug(`Ignoring ${cmdName} command`);
		// ...
		// ALIAS name ANYTHING ELSE THAT MIGHT BE PART OF THE COMMAND...
	} else if cmdName == "ALIAS" {

		// FOR <varName> IN <expression>
		// IF <expression>
	} else if cmdName == "FOR" || cmdName == "IF" {
		processForIf(data, node, ctx, cmd, cmdName, rest)

		// END-FOR
		// END-IF
	} else if cmdName == "END-FOR" || cmdName == "END-IF" {
		processEndForIf(node, ctx, cmd, cmdName, rest)

		// INS <expression>
	} else if cmdName == "INS" {
		if !isLoopExploring(ctx) {

			var varValue VarValue
			var exists bool

			if rest[0] != '$' {
				varValue, exists = data.GetValue(rest)
			} else {
				splited := strings.Split(rest, ".")

				varValue, exists = ctx.vars[splited[0]]
				if exists && len(splited) == 2 {
					reflectValue := reflect.ValueOf(varValue)
					if reflectValue.Kind() == reflect.Map {
						reflectKey := reflect.ValueOf(splited[1])
						if fieldValue := reflectValue.MapIndex(reflectKey); fieldValue.IsValid() {
							varValue = fieldValue
						} else {
							exists = false
						}
					} else {
						exists = false

					}
				}
			}

			var value string
			if exists {
				value = fmt.Sprintf("%v", varValue)
			} else if ctx.options.ErrorHandler != nil {
				value = ctx.options.ErrorHandler(errors.New("KeyNotFound: "+rest), rest)
			}

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
	} else if cmdName == "EXEC" {
	} else if cmdName == "IMAGE" {
	} else if cmdName == "LINK" {
	} else if cmdName == "HTML" {
	} else {
		return "", errors.New("CommandSyntaxError: " + cmd)
	}

	return "", nil
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
func walkTemplate(data ReportData, template Node, ctx *Context, processor CommandProcessor) (report *ReportOutput, retErr error) {
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

		slog.Debug(`Next node`, "move", move, "level", ctx.level, "nodeIn", debugPrintNode(nodeIn))

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
					parent.SetChildren(append(parent.Children(), &imgNode))
					if len(captionNodes) > 0 {
						for _, captionNode := range captionNodes {
							captionNode.SetParent(parent)
							parent.SetChildren(append(parent.Children(), &captionNode))
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
			if ctx.pendingLinkNode != nil && isNotTextNode && nonTextNodeOut.Tag == "r" {
				linkNode := ctx.pendingLinkNode
				parent := nodeOut.Parent()
				if parent != nil {
					linkNode.SetParent(parent)
					// pop last children
					parent.PopChild()
					parent.SetChildren(append(parent.Children(), linkNode))
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
					//updateID(newNode.(*NonTextNode), ctx)
				}
			}

			// If it's a text node inside a w:t, process it
			parent := nodeIn.Parent()
			nodeInTxt, isNodeInTxt := nodeIn.(*TextNode)
			nodeInParentNTxt, isNodeInParentNTxt := parent.(*NonTextNode)
			if isNodeInTxt && parent != nil && isNodeInParentNTxt && nodeInParentNTxt.Tag == T_TAG {
				result, err := processText(&data, nodeInTxt, ctx, processor)
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
	}, nil

}

func processText(data *ReportData, node *TextNode, ctx *Context, onCommand CommandProcessor) (string, error) {
	cmdDelimiter := ctx.options.CmdDelimiter
	failFast := ctx.options.FailFast

	text := node.Text
	if text == "" {
		return "", nil
	}

	segments := splitTextByDelimiters(text, cmdDelimiter)
	outText := ""
	errorsList := []error{}

	for idx, segment := range segments {
		if idx > 0 {
			// Include the separators in the `buffers` field (used for deleting paragraphs if appropriate)
			appendTextToTagBuffers(cmdDelimiter[0], ctx, map[string]bool{"fCmd": true})
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
				if strings.Contains(segment, "finition") {
					slog.Debug("Here")
				}
				cmdResultText, err := onCommand(*data, node, ctx)
				if err != nil {
					if failFast {
						return "", err
					} else {
						errorsList = append(errorsList, err)
					}
				} else {
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

func splitTextByDelimiters(text string, delimiters [2]string) []string {
	segments := strings.Split(text, delimiters[0])
	var result []string
	for _, seg := range segments {
		result = append(result, strings.Split(seg, delimiters[1])...)
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

func updateID(node *NonTextNode, ctx *Context) {
	ctx.imageAndShapeIdIncrement += 1
	id := fmt.Sprint(ctx.imageAndShapeIdIncrement)
	node.Attrs = map[string]string{
		"id": id,
	}
}

func NewContext(options CreateReportOptions, imageAndShapeIdIncrement int) Context {

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
