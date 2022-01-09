package fileServer

import (
  "crypto/md5"
  "crypto/sha256"
  "encoding/json"
  "fmt"
  "hash"
  "io/ioutil"
  "log"
  "net/http"
  "os"
  "strings"

  "github.com/google/uuid"
  "github.com/Thomas-Redding/go_util/disk"
  "github.com/Thomas-Redding/go_util/network"
)





/********** ChildFileServer **********/

type ChildFileServer struct {
  parent *ParentFileServer
  routineId string
}

func (cfs *ChildFileServer) Lock(paths []string) error {
  if cfs.parent.loggingEnabled > 0 {
    log.Println("FileServer.go", "Lock", paths)
  }
  for i, path := range paths {
    if strings.HasPrefix(path, "/") {
      path = path[1:]
    }
    paths[i] = cfs.parent.rootDir + path
  }
  if cfs.parent.loggingEnabled > 1 {
    log.Println("FileServer.go", "LockB", paths)
  }
  return cfs.parent.scheduler.WaitUntilAllAvailableUrgent(cfs.routineId, paths)
}

func (cfs *ChildFileServer) Unlock(paths []string) {
  if cfs.parent.loggingEnabled > 0 {
    log.Println("FileServer.go", "Unlock", paths)
  }
  for i, path := range paths {
    if strings.HasPrefix(path, "/") {
      path = path[1:]
    }
    paths[i] = cfs.parent.rootDir + path
  }
  if cfs.parent.loggingEnabled > 1 {
    log.Println("FileServer.go", "UnlockB", paths)
  }
  cfs.parent.scheduler.DoneAll(cfs.routineId, paths)
}

/*
 * The below methods simply wrap the `disk` methods with Lock() and Unlock()
 */

func (cfs *ChildFileServer) ChildrenOfDir(dirPath string) ([]string, error) {
  dirPath = cfs.parent.rootDir + dirPath
  cfs.Lock([]string{dirPath})
  defer cfs.Unlock([]string{dirPath})
  return disk.ChildrenOfDir(dirPath)
}

func (cfs *ChildFileServer) Copy(fromPath string, toPath string) error {
  fromPath = cfs.parent.rootDir + fromPath
  toPath = cfs.parent.rootDir + toPath
  cfs.Lock([]string{fromPath, toPath})
  defer cfs.Unlock([]string{fromPath, toPath})
  return disk.Copy(fromPath, toPath)
}

func (cfs *ChildFileServer) CopyFile(fromPath string, toPath string) error {
  fromPath = cfs.parent.rootDir + fromPath
  toPath = cfs.parent.rootDir + toPath
  cfs.Lock([]string{fromPath, toPath})
  defer cfs.Unlock([]string{fromPath, toPath})
  return disk.CopyFile(fromPath, toPath)
}

func (cfs *ChildFileServer) CopyDir(fromPath string, toPath string) error {
  fromPath = cfs.parent.rootDir + fromPath
  toPath = cfs.parent.rootDir + toPath
  cfs.Lock([]string{fromPath, toPath})
  defer cfs.Unlock([]string{fromPath, toPath})
  return disk.CopyDir(fromPath, toPath)
}

func (cfs *ChildFileServer) Exists(path string) (bool, error) {
  path = cfs.parent.rootDir + path
  cfs.Lock([]string{path})
  defer cfs.Unlock([]string{path})
  return disk.Exists(path)
}

func (cfs *ChildFileServer) FileContentType(filePath string) (string, error) {
  filePath = cfs.parent.rootDir + filePath
  cfs.Lock([]string{filePath})
  defer cfs.Unlock([]string{filePath})
  return disk.FileContentType(filePath)
}

func (cfs *ChildFileServer) FileHash(filePath string, hasher hash.Hash) (string, error) {
  filePath = cfs.parent.rootDir + filePath
  cfs.Lock([]string{filePath})
  defer cfs.Unlock([]string{filePath})
  return disk.FileHash(filePath, hasher)
}

func (cfs *ChildFileServer) IsDirFile(path string) (bool, bool, error) {
  path = cfs.parent.rootDir + path
  cfs.Lock([]string{path})
  defer cfs.Unlock([]string{path})
  return disk.IsDirFile(path)
}

func (cfs *ChildFileServer) Unzip(zipFilePath string, destinationPath string) error {
  zipFilePath = cfs.parent.rootDir + zipFilePath
  destinationPath = cfs.parent.rootDir + destinationPath
  cfs.Lock([]string{zipFilePath, destinationPath})
  defer cfs.Unlock([]string{zipFilePath, destinationPath})
  return disk.Unzip(zipFilePath, destinationPath)
}

func (cfs *ChildFileServer) ZipFile(filePath string, zipFilePath string) error {
  filePath = cfs.parent.rootDir + filePath
  zipFilePath = cfs.parent.rootDir + zipFilePath
  cfs.Lock([]string{filePath, zipFilePath})
  cfs.Unlock([]string{filePath, zipFilePath})
  return disk.ZipFile(filePath, zipFilePath)
}

func (cfs *ChildFileServer) ZipDir(dirPath string, zipFilePath string) error {
  dirPath = cfs.parent.rootDir + dirPath
  zipFilePath = cfs.parent.rootDir + zipFilePath
  cfs.Lock([]string{dirPath, zipFilePath})
  cfs.Unlock([]string{dirPath, zipFilePath})
  return disk.ZipDir(dirPath, zipFilePath)
}


/*
 * Handle a request from FileUtil.py with appropriate locking.
 */

func (cfs *ChildFileServer) Handle(writer http.ResponseWriter, request *http.Request) {
  if cfs.parent.loggingEnabled > 0 {
    log.Println("FileServer.go", "Handle", request.Method, request.URL.Path)
  }
  if !strings.HasPrefix(request.URL.Path, cfs.parent.urlPrefix) {
    cfs.sendError(writer, 500, "Internal Server Error: Wrong Prefix")
    return
  }

  path, err := cfs.filePathFromURLPath(request.URL.Path)
  if err != nil {
    cfs.sendError(writer, 400, "Bad Request")
    return
  }

  if request.Method == http.MethodGet || request.Method == http.MethodHead {
    neededPath, err := cfs.uniquePathFromURLPath(request.URL.Path)
    if err != nil {
      cfs.sendError(writer, 400, "Bad Request: %v", err)
      return
    }
    cfs.parent.scheduler.WaitUntilAvailable(cfs.routineId, neededPath)
    defer cfs.parent.scheduler.Done(cfs.routineId, neededPath)
    // Send the requested file.
    file, err := os.Open(path)
    if err != nil {
      cfs.sendError(writer, 404, "File Not Found: %v", err)
      return
    }
    defer file.Close()

    fileInfo, err := file.Stat();
    if err != nil {
      cfs.sendError(writer, 500, "Internal Server Error: %v", err)
      return
    }

    if !fileInfo.Mode().IsDir() {
      http.ServeFile(writer, request, path)
      return
    }

    // The path is a directory.
    // http.ServeFile() adds links to directories, but we want only plain text.
    if !strings.HasSuffix(path, "/") {
      http.Redirect(writer, request, request.URL.Path + "/", http.StatusSeeOther)
      return
    }

    response, err := childrenOfDirText(path)
    if err != nil {
      cfs.sendError(writer, 500, "Internal Server Error: %v", err)
      return
    }
    data := []byte(response)
    writer.Header().Set("Content-Type", "text/plain")
    writer.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
    writer.WriteHeader(200)
    if request.Method == http.MethodGet {
      writer.Write(data)
    } else {
      writer.Write([]byte(""))
    }
    return
  } else if request.Method == http.MethodPut {
    neededPath, err := cfs.uniquePathFromURLPath(request.URL.Path)
    if err != nil {
      cfs.sendError(writer, 400, "Bad Request: %v", err)
      return
    }
    cfs.parent.scheduler.WaitUntilAvailable(cfs.routineId, neededPath)
    defer cfs.parent.scheduler.Done(cfs.routineId, neededPath)
    err = network.SaveRequestBodyAsFile(request, path, false)
    if err != nil {
      cfs.sendError(writer, 500, "Internal Server Error: %v", err)
      return
    }
    cfs.sendError(writer, 200, "")
    return
  } else if request.Method == http.MethodDelete {
    neededPath, err := cfs.uniquePathFromURLPath(request.URL.Path)
    if err != nil {
      cfs.sendError(writer, 400, "Bad Request: %v", err)
      return
    }
    cfs.parent.scheduler.WaitUntilAvailable(cfs.routineId, neededPath)
    defer cfs.parent.scheduler.Done(cfs.routineId, neededPath)
    err = os.RemoveAll(path)
    if err != nil {
      cfs.sendError(writer, 500, "Internal Server Error: %v", err)
      return
    }
    if path == cfs.parent.rootDir {
      // If we just deleted the root directory, re-create it.
      err = os.Mkdir(path, os.ModePerm)
      if err != nil {
        cfs.sendError(writer, 500, "Internal Server Error: %v", err)
        return
      }
    }
    cfs.sendError(writer, 200, "")
    return
  } else if request.Method == http.MethodPost {
    neededPath, err := cfs.uniquePathFromURLPath(request.URL.Path)
    if err != nil {
      cfs.sendError(writer, 400, "Bad Request: %v", err)
      return
    }
    cfs.parent.scheduler.WaitUntilAvailable(cfs.routineId, neededPath)
    defer cfs.parent.scheduler.Done(cfs.routineId, neededPath)
    _, err = network.SaveFormPostAsFiles(request, path, 10 << 30) // Size limit of 10 GB
    if err != nil {
      cfs.sendError(writer, 500, "Internal Server Error: %v", err)
      return
    }
    cfs.sendError(writer, 200, "")
    return
  } else if request.Method == http.MethodPatch {
    // We misappropriate "PATCH" requests to perform various "commands" server-side.
    data, err := ioutil.ReadAll(request.Body)
    if err != nil {
      cfs.sendError(writer, 500, "Internal Server Error: %v", err)
      return
    }
    var patchRequestBody PatchRequestBody
    json.Unmarshal(data, &patchRequestBody)
    if cfs.parent.loggingEnabled > 0 {
      log.Println("ParentFileServer.go", "PATCH Body:", patchRequestBody)
    }
    path1, err := cfs.uniquePathFromURLPath(request.URL.Path)
    if err != nil {
      cfs.sendError(writer, 400, "Bad Request: %v", err)
      return
    }
    var path2 string
    if len(patchRequestBody.OtherPath) > 0 {
      path2, err = cfs.uniquePathFromURLPath(patchRequestBody.OtherPath)
      if err != nil {
        cfs.sendError(writer, 400, "Bad Request: %v", err)
        return
      }
    }
    var neededPaths []string
    if len(path2) > 0 {
      neededPaths = []string{path1, path2}
    } else {
      neededPaths = []string{path1}
    }
    cfs.parent.scheduler.WaitUntilAllAvailable(cfs.routineId, neededPaths)
    defer cfs.parent.scheduler.DoneAll(cfs.routineId, neededPaths)
    if patchRequestBody.Command == "-d" {
      dir, _, err := cfs.IsDirFile(path)
      if err != nil {
        cfs.sendError(writer, 500, "Internal Server Error: %v", err)
        return
      }
      if dir {
        cfs.sendError(writer, 200, "1")
        return
      } else {
        cfs.sendError(writer, 200, "")
        return
      }
    } else if patchRequestBody.Command == "mv" {
      otherPath, err := cfs.filePathFromURLPath(patchRequestBody.OtherPath)
      if err != nil {
        cfs.sendError(writer, 400, "Bad Request: %v", err)
        return
      }
      err = os.Rename(path, otherPath)
      if err != nil {
        cfs.sendError(writer, 500, "Internal Server Error: %v", err)
        return
      }
      cfs.sendError(writer, 200, "")
      return
    } else if (patchRequestBody.Command == "cp") {
      otherPath, err := cfs.filePathFromURLPath(patchRequestBody.OtherPath)
      if err != nil {
        cfs.sendError(writer, 400, "Bad Request: %v", err)
        return
      }
      err = cfs.Copy(path, otherPath)
      if err != nil {
        cfs.sendError(writer, 500, "Internal Server Error: %v", err)
        return
      }
      cfs.sendError(writer, 200, "")
      return
    } else if (patchRequestBody.Command == "zip") {
      otherPath, err := cfs.filePathFromURLPath(patchRequestBody.OtherPath)
      if err != nil {
        cfs.sendError(writer, 400, "Bad Request: %v", err)
        return
      }
      if !strings.HasSuffix(otherPath, ".zip") {
        cfs.sendError(writer, 400, "Bad Request: second path must end in \".zip\"")
        return
      }
      doesExist, err := cfs.Exists(otherPath)
      if err != nil {
        cfs.sendError(writer, 500, "Internal Server Error: %v", err)
        return
      }
      if doesExist {
        cfs.sendError(writer, 400, "Bad Request: Item exists at path.")
        return
      }
      dir, _, err := cfs.IsDirFile(path)
      if err != nil {
        cfs.sendError(writer, 500, "Internal Server Error: %v", err)
        return
      }
      if dir {
        err = disk.ZipDir(path, otherPath)
        if err != nil {
          cfs.sendError(writer, 500, "Internal Server Error: %v", err)
          return
        }
        cfs.sendError(writer, 200, "")
        return
      } else {
        err = disk.ZipFile(path, otherPath)
        if err != nil {
          cfs.sendError(writer, 500, "Internal Server Error: %v", err)
          return
        }
        cfs.sendError(writer, 200, "")
        return
      }
    } else if (patchRequestBody.Command == "unzip") {
      if !strings.HasSuffix(path, ".zip") {
        cfs.sendError(writer, 400, "Bad Request: second path must end in \".zip\"")
        return
      }
      otherPath, err := cfs.filePathFromURLPath(patchRequestBody.OtherPath)
      if err != nil {
        cfs.sendError(writer, 400, "Bad Request: %v", err)
        return
      }
      doesExist, err := cfs.Exists(otherPath)
      if err != nil {
        cfs.sendError(writer, 500, "Internal Server Error: %v", err)
        return
      }
      if doesExist {
        cfs.sendError(writer, 400, "Bad Request: entity exists at destination")
        return
      }
      err = disk.Unzip(path, otherPath)
      if err != nil {
        cfs.sendError(writer, 400, "Internal Server Error: %v", err)
        return
      }
      cfs.sendError(writer, 200, "")
      return
    } else if (patchRequestBody.Command == "ls") {
      response, err := childrenOfDirText(path)
      if err != nil {
        cfs.sendError(writer, 500, "Internal Server Error: %v", err)
        return
      }
      data := []byte(response)
      writer.Header().Set("Content-Type", "text/plain")
      writer.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
      writer.WriteHeader(200)
      writer.Write(data)
      return
    } else if patchRequestBody.Command == "mkdir" {
      err := os.Mkdir(path, 0755)
      if err != nil {
        cfs.sendError(writer, 500, "Internal Server Error: %v", err)
        return
      }
      cfs.sendError(writer, 200, "")
      return
    } else if patchRequestBody.Command == "md5" || patchRequestBody.Command == "sha256" {
      var val string
      if patchRequestBody.Command == "md5" {
        val, err = disk.FileHash(path, md5.New())
      } else {
        val, err = disk.FileHash(path, sha256.New())
      }
      if err != nil {
        cfs.sendError(writer, 500, "Internal Server Error: %v", err)
        return
      } else {
        cfs.sendError(writer, 200, "%s", val)
        return
      }
    } else {
      cfs.sendError(writer, 400, "Bad Request: Unsupported PATCH command")
      return
    }
  } else {
    cfs.sendError(writer, 400, "Bad Request: Unsupported Method")
    return
  }
  cfs.sendError(writer, 500, "Internal Server Error: Could not handle.")
  return
}

func (cfs *ChildFileServer) sendError(writer http.ResponseWriter, errorCode int, format string, args ...interface{}) {
  if cfs.parent.loggingEnabled > 0 {
    log.Println("ParentFileServer.go", errorCode, fmt.Sprintf(format, args...))
  }
  http.Error(writer, fmt.Sprintf(format, args...), errorCode)
}

func (cfs *ChildFileServer) filePathFromURLPath(urlPath string) (string, error) {
  uniquePath, err := cfs.uniquePathFromURLPath(urlPath)
  if err != nil {
    return "", err
  }
  return cfs.parent.rootDir + uniquePath, nil
}

func (cfs *ChildFileServer) uniquePathFromURLPath(urlPath string) (string, error) {
  if !strings.HasPrefix(urlPath, cfs.parent.urlPrefix) {
    return "", fmt.Errorf("Path did not start with url prefix: %s", urlPath)
  }
  return urlPath[len(cfs.parent.urlPrefix):], nil
}





/********** ParentFileServer **********/

type PatchRequestBody struct {
  Command string `json:"command"`
  OtherPath string `json:"otherPath"`
}

type ParentFileServer struct {
  scheduler Scheduler
  rootDir string
  urlPrefix string
  loggingEnabled uint
}

func MakeParentFileServer(rootDir string, urlPrefix string) (*ParentFileServer, error) {
  if ! strings.HasPrefix(urlPrefix, "/") {
    return nil, fmt.Errorf("URL prefix doesn't start in a slash.")
  }
  if ! strings.HasSuffix(urlPrefix, "/") {
    return nil, fmt.Errorf("URL prefix doesn't end in a slash.")
  }
  if ! strings.HasSuffix(rootDir, "/") {
    return nil, fmt.Errorf("Root path doesn't end in a slash.")
  }
  return &ParentFileServer{
    scheduler: MakeScheduler(),
    rootDir: rootDir,
    urlPrefix: urlPrefix,
  }, nil
}

func (pfs *ParentFileServer) NewRoutine() *ChildFileServer {
  return &ChildFileServer{parent: pfs, routineId: uuid.NewString()}
}

// 0 = none
// 1 = API calls
// 2 = info logs
// 3 = debug logs
func (pfs *ParentFileServer) GetLoggingEnabled() uint {
  if pfs.loggingEnabled > 0 {
    log.Println("ParentFileServer.go", "GetLoggingEnabled")
  }
  return pfs.loggingEnabled
}

func (pfs *ParentFileServer) SetLoggingEnabled(loggingEnabled uint) {
  if pfs.loggingEnabled > 0 {
    log.Println("ParentFileServer.go", "SetLoggingEnabled")
  }
  pfs.loggingEnabled = loggingEnabled
  pfs.scheduler.loggingEnabled = loggingEnabled
}





/********** Classless Functions **********/

func childrenOfDirText(path string) (string, error) {
  children, err := disk.ChildrenOfDir(path)
  if err != nil {
    return "", err
  }
  response := ""
  for _, childName := range children {
    if strings.HasPrefix(childName, ".") { continue }
    if response != "" {
      response = response + "\n"
    }
    response = response + childName
  }
  return response, nil
}
