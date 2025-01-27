package main

import (
	"archive/zip"
	"fmt"
	"io"
	"log/slog"

	. "github.com/ArFnds/godocx-template/internal"
)

const (
	DOCUMENT_PATH = "word/document.xml"
)

func ZipGetText(z *zip.ReadCloser, filename string) (string, error) {
	rc, err := z.Open(filename)
	if err != nil {
		return "", err
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func main() {
	slog.SetLogLoggerLevel(slog.LevelDebug)
	// open the defaultTemplate as a zip file
	zip, err := zip.OpenReader("defaultTemplate.docx")
	if err != nil {
		panic(err)
	}
	defer zip.Close()

	// open the main document
	doc, err := ZipGetText(zip, DOCUMENT_PATH)
	if err != nil {
		panic(err)
	}

	// xml parse the document
	root, err := ParseXml(doc)
	if err != nil {
		panic(err)
	}

	prenode, err := PreprocessTemplate(root, []string{"+++", "+++"})
	if err != nil {
		panic(err)
	}
	DisplayContent(prenode)

}

func DisplayContent(node Node) {
	switch n := node.(type) {
	case *NonTextNode:
		for _, child := range n.ChildNodes {
			DisplayContent(child)
		}
	case *TextNode:
		fmt.Println(n.Text)
	}
}
