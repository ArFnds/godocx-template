package internal

import (
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
)

func getValueFrom(key string, source map[string]any) (VarValue, bool) {

	splitted := strings.Split(key, ".")
	idx := 0
	var loopMap map[string]any = source
	for idx < len(splitted) {
		optional := false
		key := splitted[idx]
		if strings.HasSuffix(key, "?") {
			optional = true
			key = strings.TrimSuffix(key, "?")
		}
		value, ok := loopMap[key]

		// Last iteration
		if ok && idx == len(splitted)-1 {
			return value, true
			// Ok, but not last iteration
		} else if ok {
			loopMap, ok = value.(map[string]any)
			if !ok {
				return "", optional
			}
			// Not ok but optional
		} else if optional {
			return "", true
			// Not ok at all
		} else {
			return "", false
		}

		idx++
	}
	return "", false

}

func AddChild(parent Node, child Node) Node {
	parent.AddChild(child)
	child.SetParent(parent)
	return child
}

// CloneNodeWithoutChildren crée une copie d'un noeud sans ses enfants
func CloneNodeWithoutChildren(node Node) Node {
	switch nd := node.(type) {
	case *NonTextNode:
		return &NonTextNode{
			BaseNode: BaseNode{
				ParentNode: node.Parent(),
			},
			Tag:   nd.Tag,
			Attrs: nd.Attrs,
		}
	case *TextNode:
		return &TextNode{
			Text: nd.Text,
		}
	}
	return nil
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
	newTNode.SetParent(tNodeParent)

	// Créer le nouveau noeud texte
	newTextNode := &TextNode{
		BaseNode: BaseNode{
			ParentNode: newTNode,
		},
		Text: "",
	}

	// Ajouter le noeud texte comme enfant du nouveau noeud w:t
	newTNode.SetChildren([]Node{newTextNode})

	// Insérer le nouveau noeud après le noeud actuel
	parent, ok := tNodeParent.(*NonTextNode)
	if !ok {
		return nil, errors.New("Template syntax error: parent node is not a non-text node")
	}

	parent.ChildNodes = append(parent.ChildNodes[:idx+1],
		append([]Node{newTNode}, parent.ChildNodes[idx+1:]...)...)

	return newTextNode, nil
}

// getNextSibling retourne le prochain noeud frère ou nil s'il n'existe pas
func getNextSibling(node Node) Node {
	parent := node.Parent()
	if parent == nil {
		return nil
	}

	siblings := parent.Children()

	idx := slices.Index(siblings, node)
	if idx < 0 || idx >= len(siblings)-1 {
		return nil
	}
	return siblings[idx+1]
}

func getCurLoop(ctx *Context) *LoopStatus {
	if len(ctx.loops) == 0 {
		return nil
	}
	return &ctx.loops[len(ctx.loops)-1]
}

func isLoopExploring(ctx *Context) bool {
	curLoop := getCurLoop(ctx)
	return curLoop != nil && curLoop.idx < 0
}

func logLoop(loops []LoopStatus) {
	if len(loops) == 0 {
		return
	}
	var level int = len(loops) - 1
	loopLevel := loops[level]

	idxStr := ""
	if loopLevel.idx >= 0 {
		idxStr = fmt.Sprint(loopLevel.idx + 1)
	} else {
		idxStr = "EXPLORATION"
	}
	builder := strings.Builder{}
	if loopLevel.isIf {
		builder.WriteString("IF")
	} else {
		builder.WriteString("FOR")
	}
	builder.WriteString(" loop on ")
	builder.WriteString(fmt.Sprint(level))
	builder.WriteString(":")
	builder.WriteString(loopLevel.varName)
	builder.WriteString(idxStr)
	builder.WriteString("/")
	builder.WriteString(fmt.Sprint(len(loopLevel.loopOver)))
	slog.Debug(builder.String())
}
