package internal

func PreprocessTemplate(root Node, delimiter []string) (Node, error) {

	node := root
	fCmd := false
	var openNode *TextNode = nil
	idxDelimiter := 0
	placeholderCmd := delimiter[0] + `CMD_NODE` + delimiter[1]

	for node != nil {
		nonTextNode, isNonTextNode := node.(*NonTextNode)
		textNode, isTextNode := node.(*TextNode)

		// Add `xml:space` attr `preserve` to `w:t` tags
		if isNonTextNode && nonTextNode.Tag == T_TAG {
			nonTextNode.Attrs["xml:space"] = Attribute{Value: "preserve"}
		}

		// Add a space if we reach a new `w:p` tag and there's an open node (hence, in a command)
		if isNonTextNode && nonTextNode.Tag == P_TAG && openNode != nil {
			openNode.Text = openNode.Text + " "
		}

		if isTextNode && node.Parent() != nil {
			if parentNode, isParentNonText := node.Parent().(*NonTextNode); isParentNonText && parentNode.Tag == T_TAG {
				if openNode == nil {
					openNode = textNode
				}
				textIn := textNode.Text
				textNode.Text = ""

				for i, c := range textIn {

					var currentDelimiter []rune
					if fCmd {
						currentDelimiter = []rune(delimiter[1])
					} else {
						currentDelimiter = []rune(delimiter[0])
					}

					if c == currentDelimiter[idxDelimiter] {
						idxDelimiter += 1

						// Finished matching delimiter? Then toggle `fCmd`,
						// add a new `w:t` + text node (either before or after the delimiter),
						// depending on the case
						if idxDelimiter == len(currentDelimiter) {
							fCmd = !fCmd
							fNodesMatch := node == openNode
							if fCmd && len(openNode.Text) > 0 {
								openNode, err := InsertTextSiblingAfter(openNode)
								if err != nil {
									return nil, err
								}
								if fNodesMatch {
									node = openNode
								}
							}
							openNode.Text += string(currentDelimiter)
							if !fCmd && i < len(textIn)-1 {
								openNode, err := InsertTextSiblingAfter(openNode)
								if err != nil {
									return nil, err
								}
								if fNodesMatch {
									node = openNode
								}
							}
							idxDelimiter = 0
							if !fCmd {
								openNode = textNode // may switch open node to the current one
							}
						}

					} else if idxDelimiter != 0 {
						//openNode._text += currentDelimiter.slice(0, idxDelimiter);
						openNode.Text += string(currentDelimiter[0:idxDelimiter])
						idxDelimiter = 0
						if !fCmd {
							openNode = textNode
						}
						openNode.Text += string(c)
					} else {
						openNode.Text += string(c)
					}
				}

				// Close the text node if nothing's pending
				// if (!fCmd && !idxDelimiter) openNode = null;
				if !fCmd && idxDelimiter == 0 {
					openNode = nil
				}

				// If text was present but not any more, add a placeholder, so that this node
				// will be purged during report generation
				//if (textIn.length && !node._text.length) node._text = placeholderCmd;
				if textIn != "" && textNode.Text == "" {
					textNode.Text = placeholderCmd
				}
			}
		}

		// // Find next node to process
		if len(node.Children()) > 0 {
			node = node.Children()[0]
		} else {
			fFound := false
			// while (node._parent != null) {
			for node.Parent() != nil {
				parent := node.Parent()
				nextSibling := getNextSibling(node)
				if nextSibling != nil {
					fFound = true
					node = nextSibling
					break
				}
				node = parent
			}

			if !fFound {
				node = nil
			}
		}
	}

	return root, nil
}
