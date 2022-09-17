
## FileScheduler

This class is effectively a very fancy system of mutexes. Essentially, it allows you to lock files and directories such that
* if a directory is locked, none of its files can be locked
* if a file is locked, none of its ancestors (directories) can be locked

To combat deadlock, it only allows each go-routine to hold one set of locks at a time. That is, if you call a "lock" function, you must call "Unlock" (which will release all your locks) before locking something new.

```
package main

import (
  "github.com/google/uuid"
  "log"
  "net/http"
  "strings"
)

var gFileScheduler *FileScheduler

func main() {
  gFileScheduler := MakeFileScheduler()
  http.HandleFunc("/", handle)
  http.ListenAndServe(":" + PORT, nil)
}

func handle(writer http.ResponseWriter, request *http.Request) {
  dirPath := "foo/" + uuid.NewString()
  // Lock the top level directory only when we need to modify it.
  gFileScheduler.Lock("foo")
  os.Mkdir(dirPath, 0755)
  gFileScheduler.Unlock()

  // Lock a sub-directory when we can to free up locks for other requests.
  gFileScheduler.Lock(dirPath)
  uploadFileForRequestToPath(request, dirPath)
  gFileScheduler.Unlock()
}
```

In addition to the constructor, this class has five methods:

* `Lock(path string) bool`
* `LockAll(paths []string) bool`
* `LockUrgent(path string) bool`
* `LockAllUrgent(paths []string) bool`
* `Unlock()`
