package internal

import (
	"archive/zip"
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
	ZipReader    *zip.ReadCloser
	ContentTypes *NonTextNode
}

func ProcessImages(images Images, documentComponent string, zip *zip.ReadCloser, zipWriter *zip.Writer) error {
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
		err = ZipSet(zipWriter, imgPath, imgData)
		if err != nil {
			return err
		}
		AddChild(rels, NewNonTextNode("Relationship", map[string]string{
			"Id":     imageId,
			"Type":   "http://schemas.openxmlformats.org/officeDocument/2006/relationships/image",
			"Target": fmt.Sprintf("media/%s", imgName),
		}, nil))
	}
	finalRelsXml := BuildXml(rels, XmlOptions{
		LiteralXmlDelimiter: DEFAULT_LITERAL_XML_DELIMITER,
	}, "")

	return ZipSet(zipWriter, relsPath, finalRelsXml)
}

func getRelsFromZip(zip *zip.ReadCloser, relsPath string) (Node, error) {
	relsXml, err := ZipGetText(zip, relsPath)
	if err != nil {
		return nil, err
	}
	if relsXml == "" {
		relsXml = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
		  <Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
		  </Relationships>`
	}
	return ParseXml(relsXml)
}

func ParseTemplate(path string) (*ParseTemplateResult, error) {
	zipTemplate, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	//defer zipTemplate.Close()

	contentTypes, err := readContentTypes(zipTemplate)
	if err != nil {
		return nil, err
	}
	mainDocument, err := getMainDoc(contentTypes)
	if err != nil {
		return nil, err
	}

	mainTemplatePath := fmt.Sprintf("%s/%s", TEMPLATE_PATH, mainDocument)

	// open the main document
	doc, err := ZipGetText(zipTemplate, mainTemplatePath)
	if err != nil {
		return nil, err
	}

	// xml parse the document
	root, err := ParseXml(doc)
	if err != nil {
		return nil, err
	}

	return &ParseTemplateResult{
		Root:         root,
		MainDocument: mainDocument,
		ZipReader:    zipTemplate,
		ContentTypes: contentTypes,
	}, nil
}

func parsePath(zipReader *zip.ReadCloser, xmlPath string) (*NonTextNode, error) {
	xmlFile, err := ZipGetText(zipReader, xmlPath)
	if err != nil {
		return nil, err
	}
	root, err := ParseXml(xmlFile)
	if err != nil {
		return nil, err
	}
	nonTextNode, ok := root.(*NonTextNode)
	if !ok {
		return nil, errors.New("root node is not a NonTextNode")
	}
	return nonTextNode, nil
}

func readContentTypes(zipReader *zip.ReadCloser) (*NonTextNode, error) {
	return parsePath(zipReader, CONTENT_TYPES_PATH)
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
