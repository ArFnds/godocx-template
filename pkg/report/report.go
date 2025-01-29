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

// CreateReport generates a report document based on a given template and data.
// It parses the template file, processes any commands within the template
// using provided data, and outputs the final document as a byte slice.
//
// Parameters:
//   - templatePath: The file path to the template document.
//   - data: A pointer to ReportData containing data to be inserted into the template.
//
// Returns:
//   - A byte slice representing the generated document.
//   - An error if any occurs during template parsing, processing, or document generation.
func CreateReport(templatePath string, data *ReportData, options CreateReportOptions) ([]byte, error) {
	// xml parse the document
	parseResult, err := internal.ParseTemplate(templatePath)
	if err != nil {
		return nil, fmt.Errorf("ParseTemplate failed: %w", err)
	}
	defer parseResult.ZipReader.Close()

	outBuffer := new(bytes.Buffer)

	writer := zip.NewWriter(outBuffer)
	defer writer.Close()

	if options.CmdDelimiter == nil {
		options.CmdDelimiter = &internal.Delimiters{
			Open:  DEFAULT_CMD_DELIMITER,
			Close: DEFAULT_CMD_DELIMITER,
		}
	}

	preppedTemplate, err := internal.PreprocessTemplate(parseResult.Root, *options.CmdDelimiter)
	if err != nil {
		panic(err)
	}

	result, err := internal.ProduceReport(data, preppedTemplate, internal.NewContext(options, 73086257))
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
