package internal

import (
	"errors"
	"fmt"
	"log/slog"
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

type CommandProcessor func(data ReportData, node Node, ctx Context) (string, error)

func ProduceReport(data ReportData, template Node, ctx Context) (*ReportOutput, error) {

	return walkTemplate(data, template, ctx, processCmd)
}

func processCmd(data ReportData, node Node, ctx Context) (string, error) {
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
func walkTemplate(data ReportData, template Node, ctx Context, processor CommandProcessor) (report *ReportOutput, retErr error) {
	out := CloneNodeWithoutChildren(template.(*NonTextNode))

	nodeIn := template
	var nodeOut Node = out
	move := ""
	deltaJump := 0

	loopCount := 0
	// TODO get from options
	maximumWalkingDepth := 1_000_000

	for {
		if loopCount == 40335 {
			fmt.Println("Loop 40335")
		}
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
			if (tag == "p" ||
				tag == "tbl" ||
				tag == "tr" ||
				tag == "tc") && isLoopExploring(ctx) {
				fRemoveNode = true
				// Delete last generated output node if the user inserted a paragraph
				// (or table row) with just a command
			} else if tag == "p" || tag == "tr" || tag == "tc" {
				buffers := ctx.buffers[tag]
				fRemoveNode = buffers.text == "" && buffers.cmds == "" && !buffers.fInsertedText
			}

			// Execute removal, if needed. The node will no longer be part of the output, but
			// the parent will be accessible from the child (so that we can still move up the tree)
			if fRemoveNode && nodeOut.Parent() != nil {
				nodeOut.Parent().SetChildren(nodeOut.Parent().Children()[:len(nodeOut.Parent().Children())-1])
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
					parent.SetChildren(parent.Children()[:len(parent.Children())-1])
					parent.SetChildren(append(parent.Children(), &imgNode))
					if len(captionNodes) > 0 {
						for _, captionNode := range captionNodes {
							captionNode.SetParent(parent)
							parent.SetChildren(append(parent.Children(), &captionNode))
						}
					}

					// Prevent containing paragraph or table row from being removed
					// TODO typing
					bp := ctx.buffers["p"]
					bp.fInsertedText = true
					ctx.buffers["p"] = bp
					btr := ctx.buffers["tr"]
					btr.fInsertedText = true
					ctx.buffers["tr"] = btr
					btc := ctx.buffers["tc"]
					btc.fInsertedText = true
					ctx.buffers["tc"] = btc
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
					parent.SetChildren(parent.Children()[:len(parent.Children())-1])
					parent.SetChildren(append(parent.Children(), linkNode))
					// Prevent containing paragraph or table row from being removed
					bp := ctx.buffers["p"]
					bp.fInsertedText = true
					ctx.buffers["p"] = bp
					btr := ctx.buffers["tr"]
					btr.fInsertedText = true
					ctx.buffers["tr"] = btr
					btc := ctx.buffers["tc"]
					btc.fInsertedText = true
					ctx.buffers["tc"] = btc
				}
				ctx.pendingLinkNode = nil
			}

			// If a html page was generated, replace the parent `w:p` node with
			// the html node
			if ctx.pendingHtmlNode != nil && isNotTextNode && nonTextNodeOut.Tag == "p" {
				htmlNode := ctx.pendingHtmlNode
				parent := nodeOut.Parent()
				if parent != nil {
					htmlNode.SetParent(parent)
					// pop last children
					parent.SetChildren(parent.Children()[:len(parent.Children())-1])
					parent.SetChildren(append(parent.Children(), htmlNode))
					// Prevent containing paragraph or table row from being removed
					bp := ctx.buffers["p"]
					bp.fInsertedText = true
					ctx.buffers["p"] = bp
					btr := ctx.buffers["tr"]
					btr.fInsertedText = true
					ctx.buffers["tr"] = btr
					btc := ctx.buffers["tc"]
					btc.fInsertedText = true
					ctx.buffers["tc"] = btc
				}
				ctx.pendingHtmlNode = nil
			}

			// `w:tc` nodes shouldn't be left with no `w:p` or 'w:altChunk' children; if that's the
			// case, add an empty `w:p` inside
			filterCase := slices.ContainsFunc(nodeOut.Children(), func(node Node) bool {
				nonTextNode, isNotTextNode := node.(*NonTextNode)
				return isNotTextNode && (nonTextNode.Tag == "p" || nonTextNode.Tag == "altChunk")
			})
			if isNotTextNode && nonTextNodeOut.Tag == "tc" && !filterCase {
				nodeOut.SetChildren(append(nodeOut.Children(), NewNonTextNode("p", nil, nil)))
			}

			// Save latest `w:rPr` node that was visited (for LINK properties)
			if isNotTextNode && nonTextNodeOut.Tag == "rPr" {
				ctx.textRunPropsNode = nonTextNodeOut
			}
			if isNotTextNode && nonTextNodeOut.Tag == "r" {
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
			if tag == "p" || tag == "tr" || tag == "tc" {
				ctx.buffers[tag] = BufferStatus{text: "", cmds: "", fInsertedText: false}
			}

			newNode := CloneNodeWithoutChildren(nodeIn)
			newNode.SetParent(nodeOut)
			nodeOut.SetChildren(append(nodeOut.Children(), newNode))

			if isNodeInNTxt {
				newNodeTag := nodeInNTxt.Tag
				if !isLoopExploring(ctx) && (newNodeTag == "docPr" || newNodeTag == "shape") {
					slog.Debug("detected a - ", "newNode", newNode)
					updateID(newNode.(*NonTextNode), ctx)
				}
			}

			// If it's a text node inside a w:t, process it
			parent := nodeIn.Parent()
			nodeInTxt, isNodeInTxt := nodeIn.(*TextNode)
			nodeInParentNTxt, isNodeInParentNTxt := parent.(*NonTextNode)
			if isNodeInTxt && parent != nil && isNodeInParentNTxt && nodeInParentNTxt.Tag == T_TAG {
				result, err := processText(&data, nodeInTxt, &ctx, processor)
				if err != nil {
					retErr = errors.Join(retErr, err)
				} else if newNodeTxt, ok := newNode.(*TextNode); ok {
					newNodeTxt.Text = result

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

var (
	IncompleteConditionalStatementError = errors.New("IncompleteConditionalStatementError")
)

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
			appendTextToTagBuffers(cmdDelimiter[0], ctx, map[string]bool{"fCmd": true})
		}
		if ctx.fCmd {
			ctx.cmd += segment
		} else if !isLoopExploring(*ctx) {
			outText += segment
		}
		appendTextToTagBuffers(segment, ctx, map[string]bool{"fCmd": ctx.fCmd})

		if idx < len(segments)-1 {

			if ctx.fCmd {
				cmdResultText, err := onCommand(*data, node, *ctx)
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

var BufferKeys []string = []string{"p", "tr", "tc"}

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
		ctx.buffers[key] = buf
	}
}

func formatErrors(errorsList []error) string {
	errMsgs := []string{}
	for _, err := range errorsList {
		errMsgs = append(errMsgs, err.Error())
	}
	return strings.Join(errMsgs, "; ")
}

func updateID(node *NonTextNode, ctx Context) {
	ctx.imageAndShapeIdIncrement += 1
	id := fmt.Sprint(ctx.imageAndShapeIdIncrement)
	node.Attrs = map[string]Attribute{
		"id": {
			Value: id,
		},
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
		buffers: map[string]BufferStatus{
			"p":  {text: "", cmds: "", fInsertedText: false},
			"tr": {text: "", cmds: "", fInsertedText: false},
			"tc": {text: "", cmds: "", fInsertedText: false},
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
