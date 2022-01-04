package fileServer

import (
  "strings"
)


type PathNode struct {
  name string
  parent *PathNode
  children map[string]*PathNode
  exists bool
}

type FileTrie struct {
  root *PathNode
  count int
}

func MakeFileTrie() FileTrie {
  return FileTrie{root: makeNode("", nil, false), count: 0}
}

func (trie *FileTrie)Add(filePath string) {
  path := strings.Split(filePath, "/")
  node := trie.root
  for i, part := range path {
  	val, ok := node.children[part]
    if ok {
      node = val
    } else {
      newNode := makeNode(part, node, i == len(path) - 1)
      trie.count += 1
      node.children[part] = newNode
      node = newNode
    }
  }
}

func (trie *FileTrie)ContainsPathOrParent(filePath string) bool {
  path := strings.Split(filePath, "/")
  node := trie.root
  for _, part := range path {
    val, ok := node.children[part]
    if ok {
      node = val
      if node.exists {
        return true;
      }
    }
  }
  return false
}

func (trie *FileTrie)Length() int {
  return trie.count
}

func (trie *FileTrie)Remove(filePath string) bool {
  path := strings.Split(filePath, "/")
  node := trie.root
  for i, part := range path {
    val, ok := node.children[part]
    if i == len(path) - 1 {
      delete(node.children, part)
      trie.count -= 1
      for node.parent != nil {
        if !node.exists && len(node.children) == 0 {
          delete(node.parent.children, node.name)
          trie.count -= 1
        }
        node = node.parent
      }
      return true;
    }
    if ok {
      node = val
    } else {
      return false;
    }
  }
  // Impossible
  return false;
}

func fullPathFromNode(node *PathNode) string {
  rtn := make([]string, 0)
  for node.parent != nil {
    rtn = append(rtn, node.name)
    node = node.parent
  }
  // Reverse array
  for i, j := 0, len(rtn)-1; i < j; i, j = i+1, j-1 {
    rtn[i], rtn[j] = rtn[j], rtn[i]
  }
  return strings.Join(rtn, "/")
}

func makeNode(name string, parent *PathNode, exists bool) *PathNode {
  return &PathNode{name: name, parent: parent, children: make(map[string]*PathNode, 0), exists: exists}
}
