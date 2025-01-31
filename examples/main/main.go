package main

import (
	"os"

	. "github.com/ArFnds/godocx-template/pkg/report"
	"github.com/skip2/go-qrcode"
)

func main() {

	outBytes, err := CreateReport(os.Args[1],
		&ReportData{},
		CreateReportOptions{
			LiteralXmlDelimiter: "||",
			// Otherwise unused but mandatory options
			ProcessLineBreaks: true,
			Functions: Functions{
				"qr": func(args ...any) VarValue {
					if url, ok := args[0].(string); ok {
						png, err := qrcode.Encode(url, qrcode.Medium, 256)
						if err != nil {
							return "Err"
						}
						return &ImagePars{
							Data:      png,
							Extension: ".png",
							Width:     6,
							Height:    6,
						}
					}
					return ""
				},
			},
		})
	if err != nil {
		panic(err)
	}

	err = os.WriteFile(os.Args[2], outBytes, 0666)
	if err != nil {
		panic(err)
	}

}
