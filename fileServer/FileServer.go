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

  "github.com/Thomas-Redding/go_util/disk"
  "github.com/Thomas-Redding/go_util/network"
)

type PatchRequestBody struct {
  Command string `json:"command"`
  OtherPath string `json:"otherPath"`
}

type FileServer struct {
  scheduler Scheduler
  rootDir string
  urlPrefix string
  LoggingEnabled bool
}

func MakeFileServer(rootDir string, urlPrefix string) (*FileServer, error) {
  if ! strings.HasPrefix(urlPrefix, "/") {
    return nil, fmt.Errorf("URL prefix doesn't start in a slash.")
  }
  if ! strings.HasSuffix(urlPrefix, "/") {
    return nil, fmt.Errorf("URL prefix doesn't end in a slash.")
  }
  if ! strings.HasSuffix(rootDir, "/") {
    return nil, fmt.Errorf("Root path doesn't end in a slash.")
  }
  return &FileServer{
    scheduler: MakeScheduler(),
    rootDir: rootDir,
    urlPrefix: urlPrefix,
  }, nil
}

func (fs *FileServer) Lock(paths []string) error {
  return fs.scheduler.WaitUntilAllAvailableUrgent(paths)
}

func (fs *FileServer) Unlock(paths []string) {
  fs.scheduler.DoneAll(paths)
}

func (fs *FileServer) Handle(writer http.ResponseWriter, request *http.Request) {
  fs.scheduler.LoggingEnabled = fs.LoggingEnabled
  if fs.LoggingEnabled {
    log.Println("FileServer.go", request.Method, request.URL.Path)
  }
  if !strings.HasPrefix(request.URL.Path, fs.urlPrefix) {
    fs.sendError(writer, 500, "Internal Server Error: Wrong Prefix")
    return
  }

  path, err := fs.filePathFromURLPath(request.URL.Path)
  if err != nil {
    fs.sendError(writer, 400, "Bad Request")
    return
  }

  if request.Method == http.MethodGet || request.Method == http.MethodHead {
    neededPath, err := fs.uniquePathFromURLPath(request.URL.Path)
    if err != nil {
      fs.sendError(writer, 400, "Bad Request: %v", err)
      return
    }
    fs.scheduler.WaitUntilAvailable(neededPath)
    defer fs.scheduler.Done(neededPath)
    // Send the requested file.
    file, err := os.Open(path)
    if err != nil {
      fs.sendError(writer, 404, "File Not Found: %v", err)
      return
    }
    defer file.Close()

    fileInfo, err := file.Stat();
    if err != nil {
      fs.sendError(writer, 500, "Internal Server Error: %v", err)
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
      fs.sendError(writer, 500, "Internal Server Error: %v", err)
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
    neededPath, err := fs.uniquePathFromURLPath(request.URL.Path)
    if err != nil {
      fs.sendError(writer, 400, "Bad Request: %v", err)
      return
    }
    fs.scheduler.WaitUntilAvailable(neededPath)
    defer fs.scheduler.Done(neededPath)
    err = network.SaveRequestBodyAsFile(request, path, false)
    if err != nil {
      fs.sendError(writer, 500, "Internal Server Error: %v", err)
      return
    }
    fs.sendError(writer, 200, "")
    return
  } else if request.Method == http.MethodDelete {
    neededPath, err := fs.uniquePathFromURLPath(request.URL.Path)
    if err != nil {
      fs.sendError(writer, 400, "Bad Request: %v", err)
      return
    }
    fs.scheduler.WaitUntilAvailable(neededPath)
    defer fs.scheduler.Done(neededPath)
    err = os.RemoveAll(path)
    if err != nil {
      fs.sendError(writer, 500, "Internal Server Error: %v", err)
      return
    }
    if path == fs.rootDir {
      // If we just deleted the root directory, re-create it.
      err = os.Mkdir(path, os.ModePerm)
      if err != nil {
        fs.sendError(writer, 500, "Internal Server Error: %v", err)
        return
      }
    }
    fs.sendError(writer, 200, "")
    return
  } else if request.Method == http.MethodPost {
    neededPath, err := fs.uniquePathFromURLPath(request.URL.Path)
    if err != nil {
      fs.sendError(writer, 400, "Bad Request: %v", err)
      return
    }
    fs.scheduler.WaitUntilAvailable(neededPath)
    defer fs.scheduler.Done(neededPath)
    err = network.SaveFormPostAsFiles(request, path, 100 << 20) // Size limit of 100 MB
    if err != nil {
      fs.sendError(writer, 500, "Internal Server Error: %v", err)
      return
    }
    fs.sendError(writer, 200, "")
    return
  } else if request.Method == http.MethodPatch {
    // We misappropriate "PATCH" requests to perform various "commands" server-side.
    data, err := ioutil.ReadAll(request.Body)
    if err != nil {
      fs.sendError(writer, 500, "Internal Server Error: %v", err)
      return
    }
    var patchRequestBody PatchRequestBody
    json.Unmarshal(data, &patchRequestBody)
    if fs.LoggingEnabled {
      log.Println("FileServer.go", "PATCH Body:", patchRequestBody)
    }
    path1, err := fs.uniquePathFromURLPath(request.URL.Path)
    if err != nil {
      fs.sendError(writer, 400, "Bad Request: %v", err)
      return
    }
    var path2 string
    if len(patchRequestBody.OtherPath) > 0 {
      path2, err = fs.uniquePathFromURLPath(patchRequestBody.OtherPath)
      if err != nil {
        fs.sendError(writer, 400, "Bad Request: %v", err)
        return
      }
    }
    var neededPaths []string
    if len(path2) > 0 {
      neededPaths = []string{path1, path2}
    } else {
      neededPaths = []string{path1}
    }
    fs.scheduler.WaitUntilAllAvailable(neededPaths)
    defer fs.scheduler.DoneAll(neededPaths)
    if patchRequestBody.Command == "-d" {
      dir, _, err := disk.IsDirFile(path)
      if err != nil {
        fs.sendError(writer, 500, "Internal Server Error: %v", err)
        return
      }
      if dir {
        fs.sendError(writer, 200, "1")
        return
      } else {
        fs.sendError(writer, 200, "")
        return
      }
    } else if patchRequestBody.Command == "mv" {
      otherPath, err := fs.filePathFromURLPath(patchRequestBody.OtherPath)
      if err != nil {
        fs.sendError(writer, 400, "Bad Request: %v", err)
        return
      }
      err = os.Rename(path, otherPath)
      if err != nil {
        fs.sendError(writer, 500, "Internal Server Error: %v", err)
        return
      }
      fs.sendError(writer, 200, "")
      return
    } else if (patchRequestBody.Command == "cp") {
      otherPath, err := fs.filePathFromURLPath(patchRequestBody.OtherPath)
      if err != nil {
        fs.sendError(writer, 400, "Bad Request: %v", err)
        return
      }
      err = copy(path, otherPath)
      if err != nil {
        fs.sendError(writer, 500, "Internal Server Error: %v", err)
        return
      }
      fs.sendError(writer, 200, "")
      return
    } else if (patchRequestBody.Command == "zip") {
      otherPath, err := fs.filePathFromURLPath(patchRequestBody.OtherPath)
      if err != nil {
        fs.sendError(writer, 400, "Bad Request: %v", err)
        return
      }
      if !strings.HasSuffix(otherPath, ".zip") {
        fs.sendError(writer, 400, "Bad Request: second path must end in \".zip\"")
        return
      }
      doesExist, err := exists(otherPath)
      if err != nil {
        fs.sendError(writer, 500, "Internal Server Error: %v", err)
        return
      }
      if doesExist {
        fs.sendError(writer, 400, "Bad Request: Item exists at path.")
        return
      }
      dir, _, err := disk.IsDirFile(path)
      if err != nil {
        fs.sendError(writer, 500, "Internal Server Error: %v", err)
        return
      }
      if dir {
        err = disk.ZipDir(path, otherPath)
        if err != nil {
          fs.sendError(writer, 500, "Internal Server Error: %v", err)
          return
        }
        fs.sendError(writer, 200, "")
        return
      } else {
        err = disk.ZipFile(path, otherPath)
        if err != nil {
          fs.sendError(writer, 500, "Internal Server Error: %v", err)
          return
        }
        fs.sendError(writer, 200, "")
        return
      }
    } else if (patchRequestBody.Command == "unzip") {
      if !strings.HasSuffix(path, ".zip") {
        fs.sendError(writer, 400, "Bad Request: second path must end in \".zip\"")
        return
      }
      otherPath, err := fs.filePathFromURLPath(patchRequestBody.OtherPath)
      if err != nil {
        fs.sendError(writer, 400, "Bad Request: %v", err)
        return
      }
      doesExist, err := exists(otherPath)
      if err != nil {
        fs.sendError(writer, 500, "Internal Server Error: %v", err)
        return
      }
      if doesExist {
        fs.sendError(writer, 400, "Bad Request: entity exists at destination")
        return
      }
      err = disk.Unzip(path, otherPath)
      if err != nil {
        fs.sendError(writer, 400, "Internal Server Error: %v", err)
        return
      }
      fs.sendError(writer, 200, "")
      return
    } else if (patchRequestBody.Command == "ls") {
      response, err := childrenOfDirText(path)
      if err != nil {
        fs.sendError(writer, 500, "Internal Server Error: %v", err)
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
        fs.sendError(writer, 500, "Internal Server Error: %v", err)
        return
      }
      fs.sendError(writer, 200, "")
      return
    } else if patchRequestBody.Command == "md5" || patchRequestBody.Command == "sha256" {
      var val string
      if patchRequestBody.Command == "md5" {
        val, err = disk.FileHash(path, md5.New())
      } else {
        val, err = disk.FileHash(path, sha256.New())
      }
      if err != nil {
        fs.sendError(writer, 500, "Internal Server Error: %v", err)
        return
      } else {
        fs.sendError(writer, 200, "%s", val)
        return
      }
    } else {
      fs.sendError(writer, 400, "Bad Request: Unsupported PATCH command")
      return
    }
  } else {
    fs.sendError(writer, 400, "Bad Request: Unsupported Method")
    return
  }
  fs.sendError(writer, 500, "Internal Server Error: Could not handle.")
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

func (fs *FileServer) sendError(writer http.ResponseWriter, errorCode int, format string, args ...interface{}) {
  if fs.LoggingEnabled {
    log.Println("FileServer.go", errorCode, fmt.Sprintf(format, args...))
  }
  http.Error(writer, fmt.Sprintf(format, args...), errorCode)
}

func (fs *FileServer) filePathFromURLPath(urlPath string) (string, error) {
  if !strings.HasPrefix(urlPath, fs.urlPrefix) {
    return "", errors.New("Path doesn't start with the correct prefix.")
  }
  return fs.rootDir + urlPath[len(fs.urlPrefix):], nil
}

func (fs *FileServer) uniquePathFromURLPath(urlPath string) (string, error) {
  if !strings.HasPrefix(urlPath, fs.urlPrefix) {
    return "", fmt.Errorf("Path did not start with url prefix: %s", urlPath)
  }
  return urlPath[len(fs.urlPrefix):], nil
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
