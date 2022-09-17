package fileScheduler

import (
  "errors"
  "fmt"
  "strings"
)

/*
 * MakeFileLocker() FileLocker
 *
 * // Reserves the item (and all its descendants) for the routine.
 * // This lock is NOT recursive.
 * Lock(path []string, routineId string) error
 *
 * // Releases the item (and all its descendants) from the routine.
 * Unlock(path []string, routineId string) error
 *
 * // Returns true iff the item is locked by a different routine.
 * Locked(path []string, routineId string) bool
 *
 * // Prints the tree.
 * Print()
 *
 * Returns the number of locked paths.
 * NumLockedPaths() int
 */

type FileLockerPathNode struct {
  name string
  parent *FileLockerPathNode
  children map[string]*FileLockerPathNode
  explicit bool // zero for all implicit nodes
  routineId string // "" for all implicit nodes
}

type FileLocker struct {
  root *FileLockerPathNode
}

func MakeFileLocker() FileLocker {
  return FileLocker{root: makeImplicitNode("", nil)}
}

func (fileLocker *FileLocker)Lock(path []string, routineId string) error {
  node := fileLocker.lockedNode(path)
  if node != nil && node.routineId != routineId {
    return errors.New("FileLocker.go: Attempted to lock from multiple routines.");
  }
  node = fileLocker.root
  for i, part := range path {
    child, ok := node.children[part]
    if ok {
      node = child
      if i == len(path) - 1 {
        return errors.New("FileLocker.go: Attempted to lock item multiple times.");
      }
    } else {
      var newNode *FileLockerPathNode
      if i == len(path) - 1 {
        newNode = makeExplicitNode(part, node, routineId)
      } else {
        newNode = makeImplicitNode(part, node)
      }
      node.children[part] = newNode
      node = newNode
    }
  }
  return nil
}

// TODO: Make this function more efficient.
func doesOtherRoutineShareNode(node *FileLockerPathNode, routineId string) bool {
  if node.routineId != "" && node.routineId != routineId {
    return true
  }
  for _, child := range node.children {
    if doesOtherRoutineShareNode(child, routineId) {
      return true
    }
  }
  return false
}

func (fileLocker *FileLocker)Unlock(path []string, routineId string) error {
  node := fileLocker.root
  for i, part := range path {
    child, ok := node.children[part]
    if ok {
      node = child
    } else {
      return errors.New("FileLocker.go: Attempted to remove an item that wasn't locked.");
    }
    if i == len(path) - 1 {
      if (node.routineId != routineId) {
        return errors.New("FileLocker.go: Attempted to remove an item from the wrong routine.");
      }
      node.explicit = false
      maybeRemoveRecentlyImplictedNode(node)
      return nil;
    }
  }
  return errors.New("FileLocker.go: This should never happen");
}

func (fileLocker *FileLocker)Locked(path []string, routineId string) bool {
  node := fileLocker.lockedNode(path)
  return node != nil && node.routineId != routineId
}

func (fileLocker *FileLocker)UnlockAll(routineId string) error {
  // TODO(b/1)
  return nil
}

func (fileLocker *FileLocker)Print() {
  print(fileLocker.root, 0)
}

func (fileLocker *FileLocker)NumLockedPaths() int {
  return numLockedPaths(fileLocker.root)
}

func numLockedPaths(node *FileLockerPathNode) int {
  rtn := 0
  if node.explicit {
    rtn += 1
  }
  for _, child := range node.children {
    rtn += numLockedPaths(child)
  }
  return rtn
}

/*
 * Fetches the explicit node if the path is locked; otherwise returns nil.
 */
func (fileLocker *FileLocker)lockedNode(path []string) *FileLockerPathNode {
  node := fileLocker.root
  if node.explicit {
    return node
  }
  for _, part := range path {
    val, ok := node.children[part]
    if !ok {
      break
    }
    node = val
    if node.explicit {
      return node
    }
  }
  return nil
}

func maybeRemoveRecentlyImplictedNode(node *FileLockerPathNode) {
  if !node.explicit && len(node.children) == 0 && node.parent != nil {
    delete(node.parent.children, node.name)
    maybeRemoveRecentlyImplictedNode(node.parent)
  }
}

// unused
func fullPathFromNode(node *FileLockerPathNode) []string {
  rtn := make([]string, 0)
  for node.parent != nil {
    rtn = append(rtn, node.name)
    node = node.parent
  }
  // Reverse array
  for i, j := 0, len(rtn)-1; i < j; i, j = i+1, j-1 {
    rtn[i], rtn[j] = rtn[j], rtn[i]
  }
  return rtn
}

func print(node *FileLockerPathNode, indents int) {
  fmt.Println(strings.Repeat(" ", 2*indents), node.name, node.explicit, node.routineId)
  for _, child := range node.children {
    print(child, indents + 1)
  }
}

func makeImplicitNode(name string, parent *FileLockerPathNode) *FileLockerPathNode {
  return &FileLockerPathNode{name: name, parent: parent, children: make(map[string]*FileLockerPathNode, 0), explicit: false, routineId: ""}
}

func makeExplicitNode(name string, parent *FileLockerPathNode, routineId string) *FileLockerPathNode {
  return &FileLockerPathNode{name: name, parent: parent, children: make(map[string]*FileLockerPathNode, 0), explicit: true, routineId: routineId}
}
