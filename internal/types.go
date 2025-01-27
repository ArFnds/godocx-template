package internal

const (
	T_TAG = "t"
	P_TAG = "p"
)

type Node interface {
	Parent() Node
	SetParent(Node)
	Children() []Node
	SetChildren([]Node)
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

func (n *BaseNode) SetChildren(children []Node) {
	n.ChildNodes = children
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
	pendingImageNode *struct {
		image   NonTextNode
		caption []NonTextNode
	}
	imageAndShapeIdIncrement int
	images                   Images
	pendingLinkNode          *NonTextNode
	linkId                   int
	links                    Links
	pendingHtmlNode          *TextNode
	htmlId                   int
	htmls                    Htmls
	vars                     map[string]VarValue
	loops                    []LoopStatus
	fJump                    bool
	shorthands               map[string]string
	options                  CreateReportOptions
	//jsSandbox                SandBox
	textRunPropsNode *NonTextNode

	pIfCheckMap  map[Node]string
	trIfCheckMap map[Node]string
}

type ErrorHandler = func(err error, rawCode string) any

type CreateReportOptions struct {
	CmdDelimiter        [2]string
	LiteralXmlDelimiter string
	ProcessLineBreaks   bool
	//noSandbox          bool
	//runJs              RunJSFunc
	//additionalJsContext Object
	FailFast                   bool
	RejectNullish              bool
	ErrorHandler               ErrorHandler
	FixSmartQuotes             bool
	ProcessLineBreaksAsNewText bool
	MaximumWalkingDepth        int
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
