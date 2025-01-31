package internal

import (
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
)

const (
	TEMPLATE_PATH                 = "word"
	CONTENT_TYPES_PATH            = "[Content_Types].xml"
	DEFAULT_LITERAL_XML_DELIMITER = "||"
)

type ParseTemplateResult struct {
	Root         Node
	MainDocument string
	Zip          *ZipArchive
	ContentTypes *NonTextNode
}

func ProcessImages(images Images, documentComponent string, zip *ZipArchive) error {
	//`Processing images for ${documentComponent}...`
	slog.Debug("Processing images for " + documentComponent + "...")
	imageIds := make([]string, len(images))

	i := 0
	for k := range images {
		imageIds[i] = k
		i++
	}
	if len(imageIds) == 0 {
		return nil
	}
	slog.Debug("Completing document.xml.rels...")
	//relsPath = `${TEMPLATE_PATH}/_rels/${documentComponent}.rels`;
	relsPath := fmt.Sprintf("%s/_rels/%s.rels", TEMPLATE_PATH, documentComponent)
	rels, err := getRelsFromZip(zip, relsPath)
	if err != nil {
		return err
	}

	for _, imageId := range imageIds {
		image := images[imageId]
		extension := image.Extension
		imgData := image.Data

		// `template_${documentComponent}_${imageId}${extension}`;
		imgName := fmt.Sprintf("template_%s_%s%s", documentComponent, imageId, extension)
		// logger.debug(`Writing image ${imageId} (${imgName})...`);
		slog.Debug("Writing image " + imageId + " (" + imgName + ")...")
		imgPath := fmt.Sprintf("%s/media/%s", TEMPLATE_PATH, imgName)
		zip.SetFile(imgPath, imgData)

		AddChild(rels, NewNonTextNode("Relationship", map[string]string{
			"Id":     imageId,
			"Type":   "http://schemas.openxmlformats.org/officeDocument/2006/relationships/image",
			"Target": fmt.Sprintf("media/%s", imgName),
		}, nil))
	}
	finalRelsXml := BuildXml(rels, XmlOptions{
		LiteralXmlDelimiter: DEFAULT_LITERAL_XML_DELIMITER,
	}, "")

	zip.SetFile(relsPath, finalRelsXml)
	return nil
}

func ProcessHtmls(htmls Htmls, documentComponent string, zip *ZipArchive) error {
	slog.Debug(`Processing htmls for ` + documentComponent + "...")
	if len(htmls) > 0 {
		slog.Debug("Completing document.xml.rels...")
		htmlFiles := make([]string, len(htmls))
		i := 0

		relsPath := TEMPLATE_PATH + "/_rels/" + documentComponent + ".rels"
		rels, err := getRelsFromZip(zip, relsPath)
		if err != nil {
			return err
		}

		for htmlId, htmlData := range htmls {
			// Replace all period characters in the filename to play nice with more picky parsers (like Docx4j)
			htmlName := fmt.Sprintf("template_%s_%s.html", strings.ReplaceAll(documentComponent, ".", "_"), htmlId)
			slog.Debug(fmt.Sprintf("Writing html %s (%s)...\n", htmlId, htmlName))
			htmlPath := fmt.Sprintf("%s/%s", TEMPLATE_PATH, htmlName)
			htmlFiles[i] = "/" + htmlPath
			i++

			zip.SetFile(htmlPath, []byte(htmlData))
			AddChild(rels, NewNonTextNode("Relationship", map[string]string{
				"Id":     htmlId,
				"Type":   "http://schemas.openxmlformats.org/officeDocument/2006/relationships/aFChunk",
				"Target": htmlName,
			}, nil))
		}

		finalRelsXml := BuildXml(rels, XmlOptions{
			LiteralXmlDelimiter: DEFAULT_LITERAL_XML_DELIMITER,
		}, "")

		zip.SetFile(relsPath, finalRelsXml)
	}
	return nil
}

func getRelsFromZip(zip *ZipArchive, relsPath string) (Node, error) {
	relsXmlBytes, err := zip.GetFile(relsPath)
	if err != nil {
		return nil, err
	}

	relsXml := string(relsXmlBytes)

	if relsXml == "" {
		relsXml = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
		  <Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
		  </Relationships>`
	}
	return ParseXml(relsXml)
}

func ParseTemplate(zip *ZipArchive) (*ParseTemplateResult, error) {

	contentTypes, err := readContentTypes(zip)
	if err != nil {
		return nil, err
	}
	mainDocument, err := getMainDoc(contentTypes)
	if err != nil {
		return nil, err
	}

	mainTemplatePath := fmt.Sprintf("%s/%s", TEMPLATE_PATH, mainDocument)

	// open the main document
	doc, err := zip.GetFile(mainTemplatePath)
	if err != nil {
		return nil, err
	}

	// xml parse the document
	root, err := ParseXml(string(doc))
	if err != nil {
		return nil, err
	}

	return &ParseTemplateResult{
		Root:         root,
		MainDocument: mainDocument,
		Zip:          zip,
		ContentTypes: contentTypes,
	}, nil
}

func parsePath(zip *ZipArchive, xmlPath string) (*NonTextNode, error) {
	xmlFile, err := zip.GetFile(xmlPath)
	if err != nil {
		return nil, err
	}
	root, err := ParseXml(string(xmlFile))
	if err != nil {
		return nil, err
	}
	nonTextNode, ok := root.(*NonTextNode)
	if !ok {
		return nil, errors.New("root node is not a NonTextNode")
	}
	return nonTextNode, nil
}

func readContentTypes(zip *ZipArchive) (*NonTextNode, error) {
	return parsePath(zip, CONTENT_TYPES_PATH)
}

func getMainDoc(contentTypes *NonTextNode) (string, error) {
	MAIN_DOC_MIMES := []string{
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml",
		"application/vnd.ms-word.document.macroEnabled.main+xml",
	}
	for _, t := range contentTypes.Children() {
		if nonTextNode, isNonTextNode := t.(*NonTextNode); isNonTextNode {
			contentType, ok := nonTextNode.Attrs["ContentType"]
			if ok && slices.Contains(MAIN_DOC_MIMES, contentType) {
				if path, ok := nonTextNode.Attrs["PartName"]; ok {
					return strings.ReplaceAll(path, "/word/", ""), nil
				}
			}
		}
	}
	return "", fmt.Errorf("TemplateParseError Could not find main document (e.g. document.xml) in %s", CONTENT_TYPES_PATH)
}
