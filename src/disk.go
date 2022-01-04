package main

import (
  "errors"
  "fmt"
  "io"
  "net/http"
  "os"
  "strings"
)

/*
 * Returns all the children of a directory.
 * @param dirPath the path to the directory
 * @returns a list of children or an error 
 */
func ChildrenOfDir(dirPath string) ([]string, error) {
  files, err := ioutil.ReadDir(path)
  if err != nil { return nil, err }
  rtn := []string{}
  for _, file := range files {
    rtn = append(rtn, file.Name())
  }
  return rtn, nil
}

func CopyFile(inPath string, outPath string) error {
  // https://opensource.com/article/18/6/copying-files-go
  inFile, err := os.Open(inPath)
  if err != nil { return err }
  defer inFile.Close()
  outFile, err := os.Create(outPath)
  if err != nil { return err }
  defer outFile.Close()
  _, err = io.Copy(outFile, inFile)
  return err
}

func CopyDir(fromPath string, toPath string) error {
  // https://stackoverflow.com/a/67980768/4004969
  if toPath[:len(fromPath)] == fromPath {
    return errors.New("Cannot copy a folder into the folder itself!")
  }

  f, err := os.Open(fromPath)
  if err != nil {
    return err
  }

  file, err := f.Stat()
  if err != nil {
    return err
  }
  if !file.IsDir() {
    return fmt.Errorf("Source " + file.Name() + " is not a directory!")
  }

  err = os.Mkdir(toPath, 0755)
  if err != nil {
    return err
  }

  files, err := ioutil.ReadDir(fromPath)
  if err != nil {
    return err
  }

  for _, f := range files {
    if f.IsDir() {
      err = copyDir(fromPath + "/" + f.Name(), toPath + "/" + f.Name())
      if err != nil {
        return err
      }
    }
    if !f.IsDir() {
      err := copyFile(fromPath + "/" + f.Name(), toPath + "/" + f.Name())
      if err != nil {
        return err
      }
    }
  }
  return nil
}

/*
 * Guess the "Content-Type" of a file based on its first 512 bytes.
 * @param filePath the file to guess the content type of.
 * @returns the content type guess or an error
 */
func FileContentType(filePath string) (string, error) {
  // https://golangcode.com/get-the-content-type-of-file/
  file, err := os.Open(filePath)
  if err != nil {
    return "", err
  }
  buffer := make([]byte, 512)
  _, err := file.Read(buffer)
  if err != nil {
    return "", err
  }
  contentType := http.DetectContentType(buffer)
  return contentType, nil
}

/*
 * Computes a hexadecimal hash of the file at the given path
 * @param filePath the file to compute the has of
 * @param hasher which hashing algorithm to use
 * @returns the hexadecimal hash or an error
 *
 * Example Usage:
 *   import (
 *     "crypto/md5"
 *     "crypto/sha256"
 *   )
 *   func main() {
 *     md5Hash, err := util.FileHash("foo.png", md5.New())
 *     sha256Hash, err := util.FileHash("foo.png", sha256.New())
 *   }
 */
func FileHash(filePath string, hasher hash.Hash) (string, error) {
  // https://stackoverflow.com/a/40436529/4004969
  file, err := os.Open(filePath)
  if err != nil {
    return "", err
  }
  defer file.Close()
  if _, err := io.Copy(hasher, file); err != nil {
    return "", err
  }
  return hex.EncodeToString(hasher.Sum(nil)), nil
}

/*
 * Checks whether a file or directory exists at the given path
 * @param path the path to check
 * @returns (whether the entity is a directory, whether the entity is a file, error)
 *
 * Exhaustive interpretations:
 * (false, false, nil)    no entity exists here
 * (true, false, nil)     a directory exists here
 * (false, true, nil)     a file exists here
 * (false, false, error)  an error occured
 */
func IsDirFile(filePath string) (bool, bool, error) {
  file, err := os.Open(filePath)
  if os.IsNotExist(err) {
    return false, false, nil
  }
  if err != nil {
    return false, false, err
  }
  defer file.Close()

  fileInfo, err := file.Stat()
  if err != nil {
    return false, false, err
  }
  rtn := fileInfo.IsDir()
  return rtn, !rtn, nil
}

/*
 * Zip a file.
 * @param filePath the file to compress
 * @param where to place the newly created ZIP file.
 * @returns an error
 */
func ZipFile(filePath string, zipFilePath string) error {
  archive, err := os.Create(zipFilePath)
  if err != nil {
    return err
  }
  defer archive.Close()
  zipWriter := zip.NewWriter(archive)
  defer zipWriter.Close()

  f1, err := os.Open(filePath)
  if err != nil {
    return err
  }
  defer f1.Close()

  w1, err := zipWriter.Create(path.Base(filePath))
  if err != nil {
    return err
  }
  if _, err := io.Copy(w1, f1); err != nil {
    return err
  }
  return nil
}

/*
 * Zip a directory.
 * @param dirPath the directory to compress
 * @param where to place the newly created ZIP file.
 * @returns an error
 */
func ZipDir(dirPath string, zipFilePath string) error {
  // https://stackoverflow.com/a/63233911/4004969
  file, err := os.Create(zipFilePath)
  if err != nil {
    return err
  }
  defer file.Close()

  w := zip.NewWriter(file)
  defer w.Close()

  walker := func(path string, info os.FileInfo, err error) error {
    if err != nil {
      return err
    }
    if info.IsDir() {
      return nil
    }
    file, err := os.Open(path)
    if err != nil {
      return err
    }
    defer file.Close()

    // Ensure that `path` is not absolute; it should not start with "/".
    // This snippet happens to work because I don't use 
    // absolute paths, but ensure your real-world code 
    // transforms path into a zip-root relative path.
    // TODO
    f, err := w.Create(path[len(dirPath):])
    if err != nil {
      return err
    }

    _, err = io.Copy(f, file)
    if err != nil {
      return err
    }

    return nil
  }
  err = filepath.Walk(dirPath, walker)
  if err != nil {
    return err
  }
  return nil
}

/*
 * Unzip a zip file.
 */
func Unzip(zipFilePath string, destinationPath string) error {
  // https://stackoverflow.com/a/24792688/4004969
  r, err := zip.OpenReader(zipFilePath)
  if err != nil {
    return err
  }
  defer func() {
    if err := r.Close(); err != nil {
      panic(err)
    }
  }()
  err = os.Mkdir(destinationPath, 0755)
  if err != nil {
    return err
  }
  // Closure to address file descriptors issue with all the deferred .Close() methods
  extractAndWriteFile := func(f *zip.File) error {
    rc, err := f.Open()
    if err != nil {
      return err
    }
    defer func() {
      if err := rc.Close(); err != nil {
        panic(err)
      }
    }()
    path := filepath.Join(destinationPath, f.Name)
    // Check for ZipSlip (Directory traversal)
    if !strings.HasPrefix(path, filepath.Clean(destinationPath) + string(os.PathSeparator)) {
      return fmt.Errorf("illegal file path: %s", path)
    }
    if f.FileInfo().IsDir() {
      os.MkdirAll(path, f.Mode())
    } else {
      os.MkdirAll(filepath.Dir(path), f.Mode())
      f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
      if err != nil {
        return err
      }
      defer func() {
        if err := f.Close(); err != nil {
          panic(err)
        }
      }()
      _, err = io.Copy(f, rc)
      if err != nil {
        return err
      }
    }
    return nil
  }
  for _, f := range r.File {
    err := extractAndWriteFile(f)
    if err != nil {
      return err
    }
  }
  return nil
}
