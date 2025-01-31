package main

import (
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"time"

	. "github.com/ArFnds/godocx-template/pkg/report"
	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
)

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
		"totalSurface": 624 + 468,
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
		"rendementValorisations": []map[string]any{
			{
				"acquerreurValue": 10,
				"annualRevenus":   50,
				"rendementTaux":   52,
				"venalValue":      58,
			},
		},
		"immeubleValorisations": []map[string]any{
			{"name": "Utile", "area": 2254, "value": 5702620},
			{"name": "Terrasse", "area": 534, "value": 405306},
		},
		"totalValorisationAmount": 5702620 + 405306,

		"surroundingPrices": []map[string]any{},

		// images
		"imgPrincipal": &ImagePars{
			Extension: ".svg",
			Data: []byte(`
<svg xmlns="http://www.w3.org/2000/svg" id="flag-icons-ax" viewBox="0 0 640 480">
  <defs>
    <clipPath id="ax-a">
      <path fill-opacity=".7" d="M106.3 0h1133.3v850H106.3z"/>
    </clipPath>
  </defs>
  <g clip-path="url(#ax-a)" transform="matrix(.56472 0 0 .56482 -60 -.1)">
    <path fill="#0053a5" d="M0 0h1300v850H0z"/>
    <g fill="#ffce00">
      <path d="M400 0h250v850H400z"/>
      <path d="M0 300h1300v250H0z"/>
    </g>
    <g fill="#d21034">
      <path d="M475 0h100v850H475z"/>
      <path d="M0 375h1300v100H0z"/>
    </g>
  </g>
</svg>
`),
			Width:  6,
			Height: 6,
		},
		"imgMatrix":           []string{},
		"imgPrincipalAxilary": []ImagePars{},
		"imgPlanCadastral": &ImagePars{
			Width:     16.88,
			Height:    23.74,
			Data:      plancadastal,
			Extension: ".png",
		},

		// conclusions
		"longConclusion":     "Dans la mesure où les méthodes retenues ont un écart significatif de 17,8% mais que l’indexation des loyers augmentent rapidement (5 à 6%/an) la méthode par capitalisation du revenu ajustée lors prochaines indexation (1er T2025) fera progresser la valeur aux alentours des 5.300.000,00€, nous pouvons déterminer avec certitude que la valeur de l’immeuble objet de la mission est comprise entre 5.300.000,00€ et 5.800.000,00€.",
		"shortConclusion":    "Il sera retenu une valeur au jour de la mission de 5.600.000,00€ (Cinq millions six cent milles €uros)",
		"generalSituation":   "**my general situation**",
		"descriptionOfImage": "**my description of image**",
		"priceDescription":   "**my price description**",
	}

	options := CreateReportOptions{
		LiteralXmlDelimiter: "||",
		// Otherwise unused but mandatory options
		ProcessLineBreaks: true,
		Functions: Functions{
			"markdownToHtml": func(args ...any) string {
				if text, ok := args[0].(string); ok {
					extensions := parser.CommonExtensions | parser.AutoHeadingIDs | parser.NoEmptyLineBeforeBlock
					p := parser.NewWithExtensions(extensions)
					doc := p.Parse([]byte(text))
					htmlFlags := html.CommonFlags | html.HrefTargetBlank | html.CompletePage
					opts := html.RendererOptions{
						Flags: htmlFlags,
					}
					renderer := html.NewRenderer(opts)
					html := markdown.Render(doc, renderer)
					return string(html)
				}
				return ""
			},
			"formatNumberToPourcent": func(args ...any) string {
				if value, ok := args[0].(int); ok {
					return fmt.Sprintf("%d %%", value)
				}
				return ""
			},
			"formatToSquareMeters": func(args ...any) string {
				if surface, ok := args[0].(int); ok {
					return fmt.Sprintf("%d m2", surface)
				}
				return ""
			},
			"formatNumberToCurrency": func(args ...any) string {
				if value, ok := args[0].(float64); ok {
					return fmt.Sprintf("%.2f €", value)
				}
				if value, ok := args[0].(int); ok {
					return fmt.Sprintf("%d €", value)
				}
				return ""
			},
			"getLabelPriceOfOneArea": func(args ...any) string {
				var area int
				var totalPrice int
				var ok bool
				if area, ok = args[0].(int); !ok {
					slog.Debug("Failed to get area", "args", args, "type", reflect.TypeOf(args[0]))
					return ""
				}
				if totalPrice, ok = args[1].(int); !ok {
					slog.Debug("Failed to get totalPrice", "args", args, "type", reflect.TypeOf(args[1]))
					return ""
				}

				if area < 0 {
					return "impossible d'avoir le prix unitaire pour 0m2 ou negative"
				}

				unitPrice := totalPrice / area

				return fmt.Sprintf("%d €", unitPrice)
			},
		},
	}

	outBuf, err := CreateReport("defaultTemplate.docx", &data, options)
	if err != nil {
		panic(err)
	}
	err = os.WriteFile("outdoc.docx", outBuf, 0644)
	if err != nil {
		panic(err)
	}

}
