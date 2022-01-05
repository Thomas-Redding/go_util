package network

import (
  "errors"
  "fmt"
  "io"
  "io/ioutil"
  "net/http"
  "os"
  "path/filepath"
)

/*
 * Synchronously forward a request to a different URL.
 * @param request the request to forward
 * @param URL the URL to forward the request to
 * @returns either the server's response or an error
 *
 * Find ForwardResponseToClient() to see how these two methods can work together.
 */
func ForwardRequestToURL(request *http.Request, URL string) (*http.Response, error) {
  proxyRequest, err := http.NewRequest(request.Method, URL, request.Body)
  if err != nil {
    return nil, fmt.Errorf("Error constructing proxy request: %v", err)
  }
  proxyRequest.Close = request.Close
  proxyRequest.Header = make(http.Header)
  for key, value := range request.Header {
    proxyRequest.Header[key] = value
  }
  httpClient := http.Client{}
  return httpClient.Do(proxyRequest)
}

/*
 * Synchronously forward a HTTP response to a writer's client.
 * @param writer the writer whose client will receive the response
 * @param response the HTTP response to send via the writer
 *
 * This method works well with ForwardRequestToURL().
 * Here is an example server that forwards all requests starting with "/api/" to "apiserver.com":
 *
 *   import (
 *     "net/http"
 *     "strings"
 *   )
 *
 *   func handler(writer http.ResponseWriter, request *http.Request) {
 *     if strings.HasPrefix(request.URL.Path, "/api/") {
 *       response, err := serverLib.ForwardRequestToURL(request, "https://apiserver.com/" + request.URL.Path[5:])
 *       if err != nil {
 *         http.Error(writer, "Error in proxy server.", http.StatusInternalServerError)
 *       } else {
 *         ForwardResponseToClient(writer, response)
 *       }
 *     } else {
 *       http.Error(400, "Request had invalid prefix.", http.StatusBadRequest)
 *     }
 *   }
 *
 *   func main() {
 *     http.HandleFunc("/", handler)
 *     http.ListenAndServe(":8051", nil)
 *   }
 *
 */
func ForwardResponseToClient(writer http.ResponseWriter, response *http.Response) {
  headersToRelay := writer.Header()
  for key, value := range response.Header {
    for _, v := range value {
      headersToRelay.Add(key, v)
    }
  }
  writer.WriteHeader(response.StatusCode)
  io.Copy(writer, response.Body)
  response.Body.Close()
}

/*
 * Save the Body of a HTTP request to disk.
 * @param request - the request whose body we are saving
 * @param filePath - the path to save the body to
 * @param overwrite - whether to overwrite if an entity already exists at filePath
 * @returns an error
 *
 * This method only reliably works on requests less than 10 MB.
 */
func SaveRequestBodyAsFile(request *http.Request, filePath string, overwrite bool) error {
  if !overwrite {
    _, err := os.Stat(filePath)
    if os.IsNotExist(err) {
      // Continue
    } else if err != nil {
      return err
    } else {
      return errors.New("File already exists")
    }
  }
  data, err := ioutil.ReadAll(request.Body)
  if err != nil {
    return err
  }
  err = ioutil.WriteFile(filePath, data, os.FileMode(0644))
  if err != nil {
    return err
  }
  return nil
}

/*
 * Saves the contents of a POST request to disk.
 * @param request the request with the POST data
 * @param dirPath the root directory to save the POST data to
 */
func SaveFormPostAsFiles(request *http.Request, dirPath string, sizeLimit int64) error {
  // https://freshman.tech/file-upload-golang/
  err := request.ParseMultipartForm(sizeLimit)
  if err != nil {
    return err
  }
  dir, file, err := isDirFile(dirPath)
  if err != nil {
    return err
  }
  if file {
    return errors.New("Internal Server Error: file exists at path")
  }
  if ! dir {
    err = os.Mkdir(dirPath, os.ModePerm)
    if err != nil {
      return err
    }
  }
  for newFileName, fileHeaders := range request.MultipartForm.File {
    for _, fileHeader := range fileHeaders {
      file, err := fileHeader.Open()
      if err != nil {
        return err
      }
      defer file.Close()
      _, err = file.Seek(0, io.SeekStart)
      if err != nil {
        return err
      }
      err = os.MkdirAll(filepath.Dir(dirPath + "/" + fileHeader.Filename), 0755)
      if err != nil {
        return err
      }
      // Note, the old file name can be found with `fileHeader.Filename`.
      f, err := os.Create(filepath.Join(dirPath, newFileName))
      if err != nil {
        return err
      }
      defer f.Close()
      _, err = io.Copy(f, file)
      if err != nil {
        return err
      }
    }
  }
  return nil
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
func isDirFile(filePath string) (bool, bool, error) {
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
