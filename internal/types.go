package internal

const (
	T_TAG = "t"
	P_TAG = "p"
)

type Node interface {
	Parent() Node
	SetParent(Node)
	Children() []Node
}

type BaseNode struct {
	ParentNode Node
	ChildNodes []Node
}

func (n *BaseNode) Parent() Node {
	return n.ParentNode
}

func (n *BaseNode) SetParent(node Node) {
	n.ParentNode = node
}

func (n *BaseNode) Children() []Node {
	return n.ChildNodes
}

type TextNode struct {
	BaseNode
	Text string
}

var _ Node = (*TextNode)(nil)

type NonTextNode struct {
	BaseNode
	Tag   string
	Attrs map[string]Attribute
}

var _ Node = (*NonTextNode)(nil)

func NewTextNode(text string) *TextNode {
	return &TextNode{
		Text: text,
	}
}

func NewNonTextNode(tag string, attrs map[string]Attribute, children []Node) *NonTextNode {
	node := &NonTextNode{
		Tag:   tag,
		Attrs: attrs,
	}
	for _, child := range children {
		child.SetParent(node)
	}
	node.ChildNodes = children
	return node
}

type Attribute struct {
	Value     string
	Extension string
}

type BufferStatus struct {
	text          string
	cmds          string
	fInsertedText bool
}

type Context struct {
	gCntIf           int
	gCntEndIf        int
	level            int
	fCmd             bool
	cmd              string
	fSeekQuery       bool
	query            string
	buffers          map[string]BufferStatus
	pendingImageNode struct {
		image   NonTextNode
		caption []NonTextNode
	}
	imageAndShapeIdIncrement int
	images                   map[string]Image
	pendingLinkNode          NonTextNode
	linkId                   int
	links                    map[string]NonTextNode
	pendingHtmlNode          TextNode
	htmlId                   int
	htmls                    map[string]TextNode
	vars                     map[string]VarValue
	loops                    []LoopStatus
	fJump                    bool
	shorthands               map[string]string
	options                  CreateReportOptions
	//jsSandbox                SandBox
	textRunPropsNode NonTextNode

	pIfCheckMap  map[Node]string
	trIfCheckMap map[Node]string
}

type ErrorHandler = func(err error, rawCode string) any

type CreateReportOptions struct {
	cmdDelimiter        [2]string
	literalXmlDelimiter string
	processLineBreaks   bool
	//noSandbox          bool
	//runJs              RunJSFunc
	//additionalJsContext Object
	failFast                   bool
	rejectNullish              bool
	errorHandler               ErrorHandler
	fixSmartQuotes             bool
	processLineBreaksAsNewText bool
	maximumWalkingDepth        int
}

type VarValue any

type Image struct {
	extension string // [".png", ".gif", ".jpg", ".jpeg", ".svg"]
	data      string
}
type Images map[string]Image

type LoopStatus struct {
	refNode      Node
	refNodeLevel int
	varName      string
	loopOver     []VarValue
	idx          int
	isIf         bool
}

type Link struct{ url string }
type Links map[string]Link
type Htmls map[string]string
