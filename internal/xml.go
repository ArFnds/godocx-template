package internal

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

func ParseXml(templateXml string) (Node, error) {
	decoder := xml.NewDecoder(bytes.NewReader([]byte(templateXml)))

	var root Node
	var currentNode Node
	var stack []Node

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("XML parsing error: %v", err)
		}

		switch t := token.(type) {
		case xml.StartElement:
			node := NewNonTextNode(t.Name.Local, parseAttributes(t.Attr), nil)

			if currentNode != nil {
				currentNode.(*NonTextNode).ChildNodes = append(currentNode.(*NonTextNode).ChildNodes, node)
				node.SetParent(currentNode)
			} else {
				root = node
			}

			stack = append(stack, node)
			currentNode = node

		case xml.EndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
				if len(stack) > 0 {
					currentNode = stack[len(stack)-1]
				}
			}

		case xml.CharData:
			if currentNode != nil {
				text := strings.TrimSpace(string(t))
				if text != "" {
					textNode := NewTextNode(text)
					currentNode.(*NonTextNode).ChildNodes = append(currentNode.(*NonTextNode).ChildNodes, textNode)
					textNode.SetParent(currentNode)
				}
			}
		}
	}

	return root, nil
}

func parseAttributes(attrs []xml.Attr) map[string]Attribute {
	attrMap := make(map[string]Attribute)
	for _, attr := range attrs {
		attrMap[attr.Name.Local] = Attribute{
			Value:       attr.Value,
			Extension:   attr.Name.Space,
			ContentType: "",
			PartName:    "",
		}
	}
	return attrMap
}
