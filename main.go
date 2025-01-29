package main

import (
	"archive/zip"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"
	"time"

	. "github.com/ArFnds/godocx-template/internal"
)

const (
	DEFAULT_CMD_DELIMITER         = "+++"
	TEMPLATE_PATH                 = "word"
	DEFAULT_LITERAL_XML_DELIMITER = "||"
	CONTENT_TYPES_PATH            = "[Content_Types].xml"
)

type ParseTemplateResult struct {
	root         Node
	mainDocument string
	zip          *zip.ReadCloser
	contentTypes *NonTextNode
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

func parseTemplate(path string) (*ParseTemplateResult, error) {
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
		root:         root,
		mainDocument: mainDocument,
		zip:          zipTemplate,
		contentTypes: contentTypes,
	}, nil
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

func processImages(images Images, documentComponent string, zip *zip.ReadCloser, zipWriter *zip.Writer) error {
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

func main() {
	slog.SetLogLoggerLevel(slog.LevelDebug)

	plancadastal, err := os.ReadFile("./demo/plan-cadastral.png")
	if err != nil {
		panic(err)
	}
	var data = ReportData{
		"dateOfDay":         time.Now().Local().Format("02/01/2006"),
		"acceptDate":        time.Now().Local().Format("02/01/2006"),
		"folderName":        "SCI RUE HENRI VIGNEAU",
		"address":           "Immeuble Trinité, commune de MERIGNAC (33700)9 Av Maurice Levy",
		"cadastralCapacity": "00 ha 33 a 69 ca",
		"cadastralSection":  "AL",
		"cadastralNumber":   "525",
		"cadastralPrefixe":  "000",
		// mission
		"missionnary":           "Monsieur Noel LORENZO",
		"entrepriseMissionnary": "SCI HENRI VIGNEAU",
		// Visite
		"inPresenceOf": "Monsieur Noel LORENZO",
		"visiteDate":   time.Now().Local().Format("02/01/2006"),
		// immeuble
		"imDescription": `Immeuble en pleine propriété comprenant :
Un bâtiment élevé sur un rez-de-chaussée et de trois étages avec roof Top.  
`,
		"imConstitution": "Au rez-de-chaussée, Un local d’activité d’environ 529m² et ses bureaux 95m² de bureaux. Au 1er étage un plateau de 468m² répartis en bureaux cloisonnés, salle de réunion, locaux sociaux et terrasses. Au 2ème étage un plateau de 675m² répartis en bureaux cloisonnés, salle de réunion, locaux sociaux et terrasses. Et au 3ème étage un plateau de 487m² avec son roof top de 312m². ",
		"locativeValue": `Le loyer de référence correspondant à cette catégorie de bien, dans le centre-ville de BORDEAUX, est évalué entre 600,00€ et 1.000,00€ par m²et par an, nous pourrons trouver des références allant jusqu’à 1.200,00€ et même un cas unique à 1.600,00€ par m et par an ² sur le secteur qui seront exclues de l’analyse. Cette valeur correspond à des prix moyens pour le parc locatif privé, toutes tailles de biens confondues, corrigée en fonction de la taille du bien et de son état général.
		En conséquence la valeur locative, qui est la contrepartie annuelle susceptible d'être obtenue sur le marché de l'usage de ce bien dans le cadre d'un contrat de location :
		 Travaux indispensables pour la location : 0 %
		 Loyer annuel HT-HC : 290.000,00 €
		 Taux de rendement brut : 5 %*
		`,
		"surfaces": []map[string]any{
			{"niveau": "Rez-de-chaussée", "total": 624, "usages": "Activité Bureau te"},
			{"niveau": "Etage 1", "total": 468, "usages": "Bureau"},
		},
		"finitions": []map[string]any{
			{"title": "Réseau informatique"},
			{"title": "Cloisons modulables"},
		},
		"commonEquipments": `Branchements eau, électricité, tout-à-l’égout. Interphone.
Entrée sécurisée avec code.
Carport voitures et vélo
Ascenseurs
Sanitaires 
`,
		"valorisations": []map[string]any{
			{
				"acquerreurValue": 10,
				"annualRevenus":   50,
				"redementTaux":    52,
				"venalValue":      58,
			},
		},
		"valorisationImmeuble": []map[string]any{
			{"name": "Utile", "area": 2254, "value": 5702620},
			{"name": "Terrasse", "area": 534, "value": 405306},
		},

		// images
		"imageMatrix":      []string{},
		"imageViewArrea23": []string{},
		"imageViewArrea1":  nil,
		"imgPlanCadastral": &ImagePars{
			Width:     16.88,
			Height:    23.74,
			Data:      plancadastal,
			Extension: ".png",
		},

		// conclusions
		"longConclusion":  "Dans la mesure où les méthodes retenues ont un écart significatif de 17,8% mais que l’indexation des loyers augmentent rapidement (5 à 6%/an) la méthode par capitalisation du revenu ajustée lors prochaines indexation (1er T2025) fera progresser la valeur aux alentours des 5.300.000,00€, nous pouvons déterminer avec certitude que la valeur de l’immeuble objet de la mission est comprise entre 5.300.000,00€ et 5.800.000,00€.",
		"shortConclusion": "Il sera retenu une valeur au jour de la mission de 5.600.000,00€ (Cinq millions six cent milles €uros)",
	}

	// xml parse the document
	parseResult, err := parseTemplate("defaultTemplate.docx")
	if err != nil {
		panic(err)
	}
	defer parseResult.zip.Close()
	// write
	outputFile, err := os.Create("outdoc.docx")
	if err != nil {
		slog.Error("Erreur lors de la création du fichier ZIP de sortie :", "err", err)
		return
	}
	defer outputFile.Close()

	writer := zip.NewWriter(outputFile)
	defer writer.Close()

	preppedTemplate, err := PreprocessTemplate(parseResult.root, []string{DEFAULT_CMD_DELIMITER, DEFAULT_CMD_DELIMITER})
	if err != nil {
		panic(err)
	}

	result, err := ProduceReport(&data, preppedTemplate, NewContext(CreateReportOptions{
		CmdDelimiter: [2]string{DEFAULT_CMD_DELIMITER, DEFAULT_CMD_DELIMITER},

		// Otherwise unused but mandatory options
		LiteralXmlDelimiter:        DEFAULT_LITERAL_XML_DELIMITER,
		ProcessLineBreaks:          true,
		FailFast:                   false,
		RejectNullish:              false,
		ErrorHandler:               nil,
		FixSmartQuotes:             false,
		ProcessLineBreaksAsNewText: false,
	}, 73086257))
	if err != nil {
		panic(err)
	}

	//numImages := len(result.Images)
	processImages(result.Images, parseResult.mainDocument, parseResult.zip, writer)

	newXml := BuildXml(result.Report, XmlOptions{
		LiteralXmlDelimiter: DEFAULT_LITERAL_XML_DELIMITER,
	}, "")

	err = ZipClone(parseResult.zip, writer, []string{
		"word/document.xml",
		"word/_rels/document.xml.rels",
	})
	if err != nil {
		fmt.Println("Erreur lors de la clonage du fichier ZIP de sortie :", err)
		return
	}

	err = ZipSet(writer, "word/document.xml", newXml)
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
