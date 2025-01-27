package internal

type ReportOutput struct {
	report Node
	images Images
	links  Links
	htmls  Htmls
}

func ProduceReport(data map[string]string, template Node, ctx Context) (ReportOutput, error) {

	return ReportOutput{
		report: template,
	}, nil
}
