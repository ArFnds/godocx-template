package internal

import (
	"reflect"
)

const (
	T_TAG        = "w:t"
	R_TAG        = "w:r"
	P_TAG        = "w:p"
	RPR_TAG      = "w:rPr"
	TBL_TAG      = "w:tbl"
	TR_TAG       = "w:tr"
	TC_TAG       = "w:tc"
	DOCPR_TAG    = "wp:docPr"
	VSHAPE_TAG   = "v:shape"
	ALTCHUNK_TAG = "w:altChunk"
)

type Node interface {
	Parent() Node
	SetParent(Node)
	Children() []Node
	SetChildren([]Node)
	PopChild()
	AddChild(Node)
	Name() string
	SetName(string)
}

type BaseNode struct {
	ParentNode Node
	ChildNodes []Node
	NodeName   string
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

func (n *BaseNode) PopChild() {
	if len(n.ChildNodes) > 0 {
		n.ChildNodes = n.ChildNodes[:len(n.ChildNodes)-1]
	}
}

func (n *BaseNode) AddChild(node Node) {
	n.ChildNodes = append(n.ChildNodes, node)
}

func (n *BaseNode) Name() string {
	return n.NodeName
}

func (n *BaseNode) SetName(name string) {
	n.NodeName = name
}

type TextNode struct {
	BaseNode
	Text string
}

var _ Node = (*TextNode)(nil)

type NonTextNode struct {
	BaseNode
	Tag   string
	Attrs map[string]string
}

var _ Node = (*NonTextNode)(nil)

func NewTextNode(text string) *TextNode {
	return &TextNode{
		Text: text,
	}
}

func NewNonTextNode(tag string, attrs map[string]string, children []Node) *NonTextNode {
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
	buffers          map[string]*BufferStatus
	pendingImageNode *struct {
		image   *NonTextNode
		caption []*NonTextNode
	}
	imageAndShapeIdIncrement int
	images                   Images
	pendingLinkNode          *NonTextNode
	linkId                   int
	links                    Links
	pendingHtmlNode          Node
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

type ErrorHandler = func(err error, rawCode string) string

type Delimiters struct {
	Open  string
	Close string
}

type Function func(args ...any) string
type Functions map[string]Function

type CreateReportOptions struct {
	CmdDelimiter        *Delimiters
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
	Functions                  Functions
}

type VarValue any

type Image struct {
	Extension string // [".png", ".gif", ".jpg", ".jpeg", ".svg"]
	Data      []byte
}
type Images map[string]*Image

var ImageExtensions []string = []string{
	".png",
	".gif",
	".jpg",
	".jpeg",
	".svg",
}

type Thumbnail struct {
	Image
	Width  int
	Height int
}
type ImagePars struct {
	Extension string // [".png", ".gif", ".jpg", ".jpeg", ".svg"]
	Data      []byte
	Width     float32
	Height    float32
	Thumbnail *Thumbnail // optional
	Alt       string     // optional
	Rotation  int        // optional
	Caption   string     // optional
}

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

func isSlice(v any) bool {
	// Utiliser reflect.TypeOf pour obtenir le type de v
	valueType := reflect.TypeOf(v)

	// Vérifier si le type est un slice
	return valueType != nil && valueType.Kind() == reflect.Slice
}
