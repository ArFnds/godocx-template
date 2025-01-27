package main

import (
	"archive/zip"
	"fmt"
	"log/slog"
	"os"

	. "github.com/ArFnds/godocx-template/internal"
)

const (
	DOCUMENT_PATH = "word/document.xml"
)

func main() {
	slog.SetLogLoggerLevel(slog.LevelDebug)
	// open the defaultTemplate as a zipTemplate file
	zipTemplate, err := zip.OpenReader("defaultTemplate.docx")
	if err != nil {
		panic(err)
	}
	defer zipTemplate.Close()

	// open the main document
	doc, err := ZipGetText(zipTemplate, DOCUMENT_PATH)
	if err != nil {
		panic(err)
	}

	// xml parse the document
	root, err := ParseXml(doc)
	if err != nil {
		panic(err)
	}

	prepedTemplate, err := PreprocessTemplate(root, []string{"+++", "+++"})
	if err != nil {
		panic(err)
	}
	DisplayContent(prepedTemplate)

	newXml := BuildXml(prepedTemplate, XmlOptions{
		LiteralXmlDelimiter: "||",
	}, "")

	// write
	outputFile, err := os.Create("out.docx")
	if err != nil {
		fmt.Println("Erreur lors de la création du fichier ZIP de sortie :", err)
		return
	}
	defer outputFile.Close()

	writer := zip.NewWriter(outputFile)
	defer writer.Close()

	err = ZipClone(zipTemplate, writer)
	if err != nil {
		fmt.Println("Erreur lors de la clonage du fichier ZIP de sortie :", err)
		return
	}

	err = ZipSetText(writer, DOCUMENT_PATH, string(newXml))
	if err != nil {
		fmt.Println("Erreur lors de la clonage du fichier ZIP de sortie :", err)
		return
	}

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
