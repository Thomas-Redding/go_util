package fileServer

import (
  "errors"
  "strings"
)

/*
 * For all operations, if `filePath` doesn't begin with a forward slash, one is
 * added.
 * 
 * ft.Add(filePath)
 * Add a file or directory to the FileTrie.
 *
 * ft.ContainsPathOrParent(filePath) bool
 * Returns `true` if and only if either the given file has been added or a
 * directory containing that file has been added.
 *
 * ft.Length() int
 * Returns the number of files in the FileTrie.
 *
 * ft.Remove(filePath) bool
 * Remove the given file from the FileTrie or silently fail.
 */

type PathNode struct {
  name string
  parent *PathNode
  children map[string]*PathNode
  count int
  value string
}

type FileTrie struct {
  root *PathNode
  count int
}

func MakeFileTrie() FileTrie {
  return FileTrie{root: makeImplicitNode("", nil), count: 0}
}

func (trie *FileTrie)Add(filePath string, value string) {
  if !strings.HasPrefix(filePath, "/") {
    filePath = "/" + filePath
  }
  path := strings.Split(filePath, "/")
  node := trie.root
  for i, part := range path {
  	val, ok := node.children[part]
    if ok {
      node = val
    } else {
      var newNode *PathNode
      if i == len(path) - 1 {
        newNode = makeExplicitNode(part, node, value)
      } else {
        newNode = makeImplicitNode(part, node)
      }
      trie.count += 1
      node.children[part] = newNode
      node = newNode
    }
  }
}

/*
 * Returns (true, node.value) for the first node in the path with node.count > 0.
 * Returns (false, "") otherwise.
 */
func (trie *FileTrie)IsPathLocked(path string) (bool, string) {
  if !strings.HasPrefix(path, "/") {
    path = "/" + path
  }
  pathArr := strings.Split(path, "/")
  node := trie.root
  for _, part := range pathArr {
    val, ok := node.children[part]
    if !ok {
      break
    }
    if val.count > 0 {
      return true, val.value
    }
    node = val
  }
  return false, ""
}

func (trie *FileTrie)Length() int {
  return trie.count
}

func (trie *FileTrie)Remove(filePath string) error {
  if !strings.HasPrefix(filePath, "/") {
    filePath = "/" + filePath
  }
  path := strings.Split(filePath, "/")
  node := trie.root
  for i, part := range path {
    val, ok := node.children[part]
    if i == len(path) - 1 {
      delete(node.children, part)
      trie.count -= 1
      for node.parent != nil {
        if node.count == 0 && len(node.children) == 0 {
          delete(node.parent.children, node.name)
          trie.count -= 1
        }
        node = node.parent
      }
      return nil;
    }
    if ok {
      node = val
    } else {
      return errors.New("FileTrie.go: Attempted to remove a file that wasn't here.");
    }
  }
  return errors.New("FileTrie.go: This should never happen");
}

func (trie *FileTrie)RemoveWhileExpectingValue(filePath string, expectedValue string) error {
  if !strings.HasPrefix(filePath, "/") {
    filePath = "/" + filePath
  }
  path := strings.Split(filePath, "/")
  node := trie.root
  for i, part := range path {
    val, ok := node.children[part]
    if i == len(path) - 1 {
      if val.value != expectedValue {
        return errors.New("Value was unexpected")
      }
      delete(node.children, part)
      trie.count -= 1
      for node.parent != nil {
        if node.count == 0 && len(node.children) == 0 {
          delete(node.parent.children, node.name)
          trie.count -= 1
        }
        node = node.parent
      }
      return nil;
    }
    if ok {
      node = val
    } else {
      return errors.New("FileTrie.go: Attempted to remove a file that wasn't here.");
    }
  }
  return errors.New("FileTrie.go: This should never happen");
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
  return "/" + strings.Join(rtn, "/")
}

func makeImplicitNode(name string, parent *PathNode) *PathNode {
  return &PathNode{name: name, parent: parent, children: make(map[string]*PathNode, 0), count: 0, value: ""}
}

func makeExplicitNode(name string, parent *PathNode, value string) *PathNode {
  return &PathNode{name: name, parent: parent, children: make(map[string]*PathNode, 0), count: 1, value: value}
}
