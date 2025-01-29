package report

import (
	"archive/zip"
	"fmt"
	"log/slog"
	"os"

	"github.com/ArFnds/godocx-template/internal"
)

const (
	DEFAULT_CMD_DELIMITER = "+++"
)

func CreateReport(data *ReportData) {
	slog.SetLogLoggerLevel(slog.LevelDebug)

	// xml parse the document
	parseResult, err := internal.ParseTemplate("defaultTemplate.docx")
	if err != nil {
		panic(err)
	}
	defer parseResult.ZipReader.Close()
	// write
	outputFile, err := os.Create("outdoc.docx")
	if err != nil {
		slog.Error("Erreur lors de la création du fichier ZIP de sortie :", "err", err)
		return
	}
	defer outputFile.Close()

	writer := zip.NewWriter(outputFile)
	defer writer.Close()

	preppedTemplate, err := internal.PreprocessTemplate(parseResult.Root, []string{DEFAULT_CMD_DELIMITER, DEFAULT_CMD_DELIMITER})
	if err != nil {
		panic(err)
	}

	result, err := internal.ProduceReport(data, preppedTemplate, internal.NewContext(internal.CreateReportOptions{
		CmdDelimiter: [2]string{DEFAULT_CMD_DELIMITER, DEFAULT_CMD_DELIMITER},

		// Otherwise unused but mandatory options
		LiteralXmlDelimiter:        internal.DEFAULT_LITERAL_XML_DELIMITER,
		ProcessLineBreaks:          true,
		FailFast:                   false,
		RejectNullish:              false,
		ErrorHandler:               nil,
		FixSmartQuotes:             false,
		ProcessLineBreaksAsNewText: false,
	}, 73086257))
	//TODO ^ max id
	if err != nil {
		panic(err)
	}

	//numImages := len(result.Images)
	internal.ProcessImages(result.Images, parseResult.MainDocument, parseResult.ZipReader, writer)

	newXml := internal.BuildXml(result.Report, internal.XmlOptions{
		LiteralXmlDelimiter: internal.DEFAULT_LITERAL_XML_DELIMITER,
	}, "")

	err = internal.ZipClone(parseResult.ZipReader, writer, []string{
		"word/document.xml",
		"word/_rels/document.xml.rels",
	})
	if err != nil {
		fmt.Println("Erreur lors de la clonage du fichier ZIP de sortie :", err)
		return
	}

	err = internal.ZipSet(writer, "word/document.xml", newXml)
	if err != nil {
		fmt.Println("Erreur lors de la clonage du fichier ZIP de sortie :", err)
		return
	}

}
