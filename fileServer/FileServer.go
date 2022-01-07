package fileServer

import (
  "crypto/md5"
  "crypto/sha256"
  "encoding/json"
  "errors"
  "fmt"
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
  return cfs.parent.lock(cfs.routineId, paths)
}

func (cfs *ChildFileServer) Unlock(paths []string) {
  cfs.parent.unlock(cfs.routineId, paths)
}

func (cfs *ChildFileServer) Handle(writer http.ResponseWriter, request *http.Request) {
  cfs.parent.handle(cfs.routineId, writer, request)
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

func (pfs *ParentFileServer) child() *ChildFileServer {
  return &ChildFileServer{parent: pfs, routineId: uuid.NewString()}
}

func (pfs *ParentFileServer) lock(routineId string, paths []string) error {
  if pfs.loggingEnabled > 0 {
    log.Println("ParentFileServer.go", "lock", paths)
  }
  return pfs.scheduler.WaitUntilAllAvailableUrgent(routineId, paths)
}

func (pfs *ParentFileServer) unlock(routineId string, paths []string) {
  if pfs.loggingEnabled > 0 {
    log.Println("ParentFileServer.go", "unlock", paths)
  }
  pfs.scheduler.DoneAll(routineId, paths)
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

func (pfs *ParentFileServer) handle(routineId string, writer http.ResponseWriter, request *http.Request) {
  if pfs.loggingEnabled > 0 {
    log.Println("ParentFileServer.go", request.Method, request.URL.Path)
  }
  if !strings.HasPrefix(request.URL.Path, pfs.urlPrefix) {
    pfs.sendError(writer, 500, "Internal Server Error: Wrong Prefix")
    return
  }

  path, err := pfs.filePathFromURLPath(request.URL.Path)
  if err != nil {
    pfs.sendError(writer, 400, "Bad Request")
    return
  }

  if request.Method == http.MethodGet || request.Method == http.MethodHead {
    neededPath, err := pfs.uniquePathFromURLPath(request.URL.Path)
    if err != nil {
      pfs.sendError(writer, 400, "Bad Request: %v", err)
      return
    }
    pfs.scheduler.WaitUntilAvailable(routineId, neededPath)
    defer pfs.scheduler.Done(routineId, neededPath)
    // Send the requested file.
    file, err := os.Open(path)
    if err != nil {
      pfs.sendError(writer, 404, "File Not Found: %v", err)
      return
    }
    defer file.Close()

    fileInfo, err := file.Stat();
    if err != nil {
      pfs.sendError(writer, 500, "Internal Server Error: %v", err)
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
      pfs.sendError(writer, 500, "Internal Server Error: %v", err)
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
    neededPath, err := pfs.uniquePathFromURLPath(request.URL.Path)
    if err != nil {
      pfs.sendError(writer, 400, "Bad Request: %v", err)
      return
    }
    pfs.scheduler.WaitUntilAvailable(routineId, neededPath)
    defer pfs.scheduler.Done(routineId, neededPath)
    err = network.SaveRequestBodyAsFile(request, path, false)
    if err != nil {
      pfs.sendError(writer, 500, "Internal Server Error: %v", err)
      return
    }
    pfs.sendError(writer, 200, "")
    return
  } else if request.Method == http.MethodDelete {
    neededPath, err := pfs.uniquePathFromURLPath(request.URL.Path)
    if err != nil {
      pfs.sendError(writer, 400, "Bad Request: %v", err)
      return
    }
    pfs.scheduler.WaitUntilAvailable(routineId, neededPath)
    defer pfs.scheduler.Done(routineId, neededPath)
    err = os.RemoveAll(path)
    if err != nil {
      pfs.sendError(writer, 500, "Internal Server Error: %v", err)
      return
    }
    if path == pfs.rootDir {
      // If we just deleted the root directory, re-create it.
      err = os.Mkdir(path, os.ModePerm)
      if err != nil {
        pfs.sendError(writer, 500, "Internal Server Error: %v", err)
        return
      }
    }
    pfs.sendError(writer, 200, "")
    return
  } else if request.Method == http.MethodPost {
    neededPath, err := pfs.uniquePathFromURLPath(request.URL.Path)
    if err != nil {
      pfs.sendError(writer, 400, "Bad Request: %v", err)
      return
    }
    pfs.scheduler.WaitUntilAvailable(routineId, neededPath)
    defer pfs.scheduler.Done(routineId, neededPath)
    _, err = network.SaveFormPostAsFiles(request, path, 10 << 30) // Size limit of 10 GB
    if err != nil {
      pfs.sendError(writer, 500, "Internal Server Error: %v", err)
      return
    }
    pfs.sendError(writer, 200, "")
    return
  } else if request.Method == http.MethodPatch {
    // We misappropriate "PATCH" requests to perform various "commands" server-side.
    data, err := ioutil.ReadAll(request.Body)
    if err != nil {
      pfs.sendError(writer, 500, "Internal Server Error: %v", err)
      return
    }
    var patchRequestBody PatchRequestBody
    json.Unmarshal(data, &patchRequestBody)
    if pfs.loggingEnabled > 0 {
      log.Println("ParentFileServer.go", "PATCH Body:", patchRequestBody)
    }
    path1, err := pfs.uniquePathFromURLPath(request.URL.Path)
    if err != nil {
      pfs.sendError(writer, 400, "Bad Request: %v", err)
      return
    }
    var path2 string
    if len(patchRequestBody.OtherPath) > 0 {
      path2, err = pfs.uniquePathFromURLPath(patchRequestBody.OtherPath)
      if err != nil {
        pfs.sendError(writer, 400, "Bad Request: %v", err)
        return
      }
    }
    var neededPaths []string
    if len(path2) > 0 {
      neededPaths = []string{path1, path2}
    } else {
      neededPaths = []string{path1}
    }
    pfs.scheduler.WaitUntilAllAvailable(routineId, neededPaths)
    defer pfs.scheduler.DoneAll(routineId, neededPaths)
    if patchRequestBody.Command == "-d" {
      dir, _, err := disk.IsDirFile(path)
      if err != nil {
        pfs.sendError(writer, 500, "Internal Server Error: %v", err)
        return
      }
      if dir {
        pfs.sendError(writer, 200, "1")
        return
      } else {
        pfs.sendError(writer, 200, "")
        return
      }
    } else if patchRequestBody.Command == "mv" {
      otherPath, err := pfs.filePathFromURLPath(patchRequestBody.OtherPath)
      if err != nil {
        pfs.sendError(writer, 400, "Bad Request: %v", err)
        return
      }
      err = os.Rename(path, otherPath)
      if err != nil {
        pfs.sendError(writer, 500, "Internal Server Error: %v", err)
        return
      }
      pfs.sendError(writer, 200, "")
      return
    } else if (patchRequestBody.Command == "cp") {
      otherPath, err := pfs.filePathFromURLPath(patchRequestBody.OtherPath)
      if err != nil {
        pfs.sendError(writer, 400, "Bad Request: %v", err)
        return
      }
      err = copy(path, otherPath)
      if err != nil {
        pfs.sendError(writer, 500, "Internal Server Error: %v", err)
        return
      }
      pfs.sendError(writer, 200, "")
      return
    } else if (patchRequestBody.Command == "zip") {
      otherPath, err := pfs.filePathFromURLPath(patchRequestBody.OtherPath)
      if err != nil {
        pfs.sendError(writer, 400, "Bad Request: %v", err)
        return
      }
      if !strings.HasSuffix(otherPath, ".zip") {
        pfs.sendError(writer, 400, "Bad Request: second path must end in \".zip\"")
        return
      }
      doesExist, err := exists(otherPath)
      if err != nil {
        pfs.sendError(writer, 500, "Internal Server Error: %v", err)
        return
      }
      if doesExist {
        pfs.sendError(writer, 400, "Bad Request: Item exists at path.")
        return
      }
      dir, _, err := disk.IsDirFile(path)
      if err != nil {
        pfs.sendError(writer, 500, "Internal Server Error: %v", err)
        return
      }
      if dir {
        err = disk.ZipDir(path, otherPath)
        if err != nil {
          pfs.sendError(writer, 500, "Internal Server Error: %v", err)
          return
        }
        pfs.sendError(writer, 200, "")
        return
      } else {
        err = disk.ZipFile(path, otherPath)
        if err != nil {
          pfs.sendError(writer, 500, "Internal Server Error: %v", err)
          return
        }
        pfs.sendError(writer, 200, "")
        return
      }
    } else if (patchRequestBody.Command == "unzip") {
      if !strings.HasSuffix(path, ".zip") {
        pfs.sendError(writer, 400, "Bad Request: second path must end in \".zip\"")
        return
      }
      otherPath, err := pfs.filePathFromURLPath(patchRequestBody.OtherPath)
      if err != nil {
        pfs.sendError(writer, 400, "Bad Request: %v", err)
        return
      }
      doesExist, err := exists(otherPath)
      if err != nil {
        pfs.sendError(writer, 500, "Internal Server Error: %v", err)
        return
      }
      if doesExist {
        pfs.sendError(writer, 400, "Bad Request: entity exists at destination")
        return
      }
      err = disk.Unzip(path, otherPath)
      if err != nil {
        pfs.sendError(writer, 400, "Internal Server Error: %v", err)
        return
      }
      pfs.sendError(writer, 200, "")
      return
    } else if (patchRequestBody.Command == "ls") {
      response, err := childrenOfDirText(path)
      if err != nil {
        pfs.sendError(writer, 500, "Internal Server Error: %v", err)
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
        pfs.sendError(writer, 500, "Internal Server Error: %v", err)
        return
      }
      pfs.sendError(writer, 200, "")
      return
    } else if patchRequestBody.Command == "md5" || patchRequestBody.Command == "sha256" {
      var val string
      if patchRequestBody.Command == "md5" {
        val, err = disk.FileHash(path, md5.New())
      } else {
        val, err = disk.FileHash(path, sha256.New())
      }
      if err != nil {
        pfs.sendError(writer, 500, "Internal Server Error: %v", err)
        return
      } else {
        pfs.sendError(writer, 200, "%s", val)
        return
      }
    } else {
      pfs.sendError(writer, 400, "Bad Request: Unsupported PATCH command")
      return
    }
  } else {
    pfs.sendError(writer, 400, "Bad Request: Unsupported Method")
    return
  }
  pfs.sendError(writer, 500, "Internal Server Error: Could not handle.")
  return
}

func copy(fromPath string, toPath string) error {
  dir, _, err := disk.IsDirFile(fromPath)
  if err != nil {
    return err
  }
  if dir {
    return disk.CopyDir(fromPath, toPath)
  } else {
    return disk.CopyFile(fromPath, toPath)
  }
}

func (pfs *ParentFileServer) sendError(writer http.ResponseWriter, errorCode int, format string, args ...interface{}) {
  if pfs.loggingEnabled > 0 {
    log.Println("ParentFileServer.go", errorCode, fmt.Sprintf(format, args...))
  }
  http.Error(writer, fmt.Sprintf(format, args...), errorCode)
}

func (pfs *ParentFileServer) filePathFromURLPath(urlPath string) (string, error) {
  if !strings.HasPrefix(urlPath, pfs.urlPrefix) {
    return "", errors.New("Path doesn't start with the correct prefix.")
  }
  return pfs.rootDir + urlPath[len(pfs.urlPrefix):], nil
}

func (pfs *ParentFileServer) uniquePathFromURLPath(urlPath string) (string, error) {
  if !strings.HasPrefix(urlPath, pfs.urlPrefix) {
    return "", fmt.Errorf("Path did not start with url prefix: %s", urlPath)
  }
  return urlPath[len(pfs.urlPrefix):], nil
}

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

func exists(path string) (bool, error) {
 _, err := os.Stat(path)
  if os.IsNotExist(err) {
    return false, nil
  }
  if err != nil {
    return false, err
  }
  return true, nil;
}
