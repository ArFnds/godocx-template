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
	Value       string
	Extension   string
	ContentType string
	PartName    string
}
