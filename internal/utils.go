package internal

import (
	"errors"
	"slices"
)

// CloneNodeWithoutChildren crée une copie d'un noeud sans ses enfants
func CloneNodeWithoutChildren(node *NonTextNode) *NonTextNode {
	return &NonTextNode{
		BaseNode: BaseNode{
			ParentNode: node.ParentNode,
		},
		Tag:   node.Tag,
		Attrs: node.Attrs,
	}
}

// InsertTextSiblingAfter crée et insère un nouveau noeud texte après le noeud texte donné
// Retourne le nouveau noeud texte ou une erreur si les conditions ne sont pas remplies
func InsertTextSiblingAfter(textNode *TextNode) (*TextNode, error) {
	// Vérifier que le noeud parent est bien un noeud w:t
	tNode, ok := textNode.ParentNode.(*NonTextNode)
	if !ok || tNode.Tag != T_TAG {
		return nil, errors.New("Template syntax error: text node not within w:t")
	}

	// Vérifier que le noeud w:t a un parent
	tNodeParent := tNode.ParentNode
	if tNodeParent == nil {
		return nil, errors.New("Template syntax error: w:t node has no parent")
	}

	// Trouver l'index du noeud w:t dans les enfants de son parent
	idx := slices.Index(tNodeParent.Children(), textNode.ParentNode)
	if idx < 0 {
		return nil, errors.New("Template syntax error: node not found in parent's children")
	}

	// Créer un nouveau noeud w:t
	newTNode := CloneNodeWithoutChildren(tNode)
	newTNode.ParentNode = tNodeParent

	// Créer le nouveau noeud texte
	newTextNode := &TextNode{
		BaseNode: BaseNode{
			ParentNode: newTNode,
		},
		Text: "",
	}

	// Ajouter le noeud texte comme enfant du nouveau noeud w:t
	newTNode.ChildNodes = []Node{newTextNode}

	// Insérer le nouveau noeud après le noeud actuel
	parent, ok := tNodeParent.(*NonTextNode)
	if !ok {
		return nil, errors.New("Template syntax error: parent node is not a non-text node")
	}

	parent.ChildNodes = append(parent.ChildNodes[:idx+1],
		append([]Node{newTNode}, parent.ChildNodes[idx+1:]...)...)

	return newTextNode, nil
}

// GetNextSibling retourne le prochain noeud frère ou nil s'il n'existe pas
func GetNextSibling(node Node) Node {
	parent := node.Parent()
	if parent == nil {
		return nil
	}

	siblings := parent.Children()
	idx := -1
	for i, sibling := range siblings {
		if sibling == node {
			idx = i
			break
		}
	}

	if idx < 0 || idx >= len(siblings)-1 {
		return nil
	}

	return siblings[idx+1]
}
