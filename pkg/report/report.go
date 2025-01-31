package report

import (
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

	outBuffer := new(bytes.Buffer)
	zip, err := internal.NewZipArchive(templatePath, outBuffer)
	if err != nil {
		return nil, err
	}

	// xml parse the document
	parseResult, err := internal.ParseTemplate(zip)
	if err != nil {
		return nil, fmt.Errorf("ParseTemplate failed: %w", err)
	}

	if options.CmdDelimiter == nil {
		options.CmdDelimiter = &internal.Delimiters{
			Open:  DEFAULT_CMD_DELIMITER,
			Close: DEFAULT_CMD_DELIMITER,
		}
	}

	preppedTemplate, err := internal.PreprocessTemplate(parseResult.Root, *options.CmdDelimiter)
	if err != nil {
		return nil, fmt.Errorf("PreprocessTemplate failed: %w", err)
	}

	result, err := internal.ProduceReport(data, preppedTemplate, internal.NewContext(options, 73086257))
	//TODO ^ max id
	if err != nil {
		return nil, fmt.Errorf("ProduceReport failed: %w", err)
	}

	err = internal.ProcessImages(result.Images, parseResult.MainDocument, parseResult.Zip)
	if err != nil {
		return nil, fmt.Errorf("ProcessImages failed: %w", err)
	}
	//err = internal.ProcessHtmls(result.Htmls, parseResult.MainDocument, parseResult.Zip)
	//if err != nil {
	//	return nil, fmt.Errorf("ProcessHtmls failed: %w", err)
	//}

	newXml := internal.BuildXml(result.Report, internal.XmlOptions{
		LiteralXmlDelimiter: internal.DEFAULT_LITERAL_XML_DELIMITER,
	}, "")

	zip.SetFile("word/document.xml", newXml)

	err = zip.Close()
	if err != nil {
		return nil, fmt.Errorf("Error closing zip : %w", err)
	}
	return outBuffer.Bytes(), nil
}
