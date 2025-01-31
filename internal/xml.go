package internal

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

type XmlOptions struct {
	LiteralXmlDelimiter string
}

func ParseXml(templateXml string) (Node, error) {
	decoder := xml.NewDecoder(bytes.NewReader([]byte(templateXml)))

	var root Node
	var currentNode Node
	var stack []Node

	for {
		token, err := decoder.RawToken()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("XML parsing error: %v", err)
		}

		switch t := token.(type) {
		case xml.StartElement:
			tag := t.Name.Local
			if t.Name.Space != "" {
				tag = t.Name.Space + ":" + t.Name.Local
			}
			node := NewNonTextNode(tag, parseAttributes(t.Attr), nil)

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
				text := string(t)
				if text != "" {
					textNode := NewTextNode(text)
					currentNode.AddChild(textNode)
					textNode.SetParent(currentNode)
				}
			}
		}
	}

	return root, nil
}

func BuildXml(node Node, options XmlOptions, indent string) []byte {
	var xmlBuffers [][]byte

	if indent == "" {
		xmlBuffers = append(xmlBuffers, []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`))
	}

	switch n := node.(type) {
	case *TextNode:
		xmlBuffers = append(xmlBuffers, []byte(sanitizeText(n.Text, options)))
	case *NonTextNode:
		var attrs strings.Builder
		for key, value := range n.Attrs {
			attrs.WriteString(fmt.Sprintf(` %s="%s"`, key, sanitizeAttr(value)))
		}

		hasChildren := len(n.Children()) > 0
		suffix := ""
		if !hasChildren {
			suffix = "/"
		}

		xmlBuffers = append(xmlBuffers, []byte(fmt.Sprintf("\n%s<%s%s%s>", indent, n.Tag, attrs.String(), suffix)))

		var lastChildIsNode bool
		for _, child := range n.Children() {
			xmlBuffers = append(xmlBuffers, BuildXml(child, options, indent+"  "))
			_, lastChildIsNode = child.(*NonTextNode)
		}

		if hasChildren {
			indent2 := ""
			if lastChildIsNode {
				indent2 = "\n" + indent
			}
			xmlBuffers = append(xmlBuffers, []byte(fmt.Sprintf("%s</%s>", indent2, n.Tag)))
		}
	}

	return bytes.Join(xmlBuffers, []byte{})
}

func parseAttributes(attrs []xml.Attr) map[string]string {
	attrMap := make(map[string]string)
	for _, attr := range attrs {
		var key string
		if attr.Name.Space == "" {
			key = attr.Name.Local
		} else {
			key = attr.Name.Space + ":" + attr.Name.Local
		}
		attrMap[key] = attr.Value
	}
	return attrMap
}

func sanitizeText(str string, options XmlOptions) string {
	var out strings.Builder
	segments := strings.Split(str, options.LiteralXmlDelimiter)
	fLiteral := false

	for _, segment := range segments {
		processedSegment := segment
		if !fLiteral {
			// Replace special characters with their XML entities
			processedSegment = strings.ReplaceAll(processedSegment, "&", "&amp;")
			processedSegment = strings.ReplaceAll(processedSegment, "<", "&lt;")
			processedSegment = strings.ReplaceAll(processedSegment, ">", "&gt;")
		}
		out.WriteString(processedSegment)
		fLiteral = !fLiteral
	}

	return out.String()
}

func sanitizeAttr(value string) string {
	// Remplace les caractères spéciaux par leurs entités XML
	value = strings.ReplaceAll(value, "&", "&amp;")
	value = strings.ReplaceAll(value, "<", "&lt;")
	value = strings.ReplaceAll(value, ">", "&gt;")
	value = strings.ReplaceAll(value, "'", "&apos;")
	value = strings.ReplaceAll(value, "\"", "&quot;")

	return value
}
