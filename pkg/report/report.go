package report

import (
	"archive/zip"
	"bytes"
	"fmt"

	"github.com/ArFnds/godocx-template/internal"
)

const (
	DEFAULT_CMD_DELIMITER = "+++"
)

func CreateReport(templatePath string, data *ReportData) ([]byte, error) {
	// xml parse the document
	parseResult, err := internal.ParseTemplate(templatePath)
	if err != nil {
		return nil, fmt.Errorf("ParseTemplate failed: %w", err)
	}
	defer parseResult.ZipReader.Close()

	outBuffer := new(bytes.Buffer)

	writer := zip.NewWriter(outBuffer)
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

	numImages := len(result.Images)
	internal.ProcessImages(result.Images, parseResult.MainDocument, parseResult.ZipReader, writer)

	newXml := internal.BuildXml(result.Report, internal.XmlOptions{
		LiteralXmlDelimiter: internal.DEFAULT_LITERAL_XML_DELIMITER,
	}, "")

	excludes := []string{
		"word/document.xml",
	}

	if numImages > 0 {
		excludes = append(excludes, "word/_rels/document.xml.rels")
	}

	err = internal.ZipClone(parseResult.ZipReader, writer, excludes)
	if err != nil {
		return nil, fmt.Errorf("Erreur lors de la clonage du fichier ZIP de sortie : %w", err)
	}

	err = internal.ZipSet(writer, "word/document.xml", newXml)
	if err != nil {
		return nil, fmt.Errorf("Erreur lors de la clonage du fichier ZIP de sortie : %w", err)
	}

	err = writer.Flush()
	if err != nil {
		return nil, fmt.Errorf("Erreur lors du flush du writer : %w", err)
	}
	err = writer.Close()
	if err != nil {
		return nil, fmt.Errorf("Erreur lors de la fermeture du writer : %w", err)
	}
	return outBuffer.Bytes(), nil
}
