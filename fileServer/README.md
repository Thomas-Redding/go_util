
## FileServer

This class is effectively a web server that needs to be wrapped by another web server. You use it by doing something like

```
package main

import (
  "log"
  "net/http"
  "strings"
)

var gFileServer *FileServer

func main() {
  rootDir := "/path/to/root/"
  urlPrefix := "/foo/"
  gFileServer, err = MakeFileServer(rootDir, urlPrefix)
  if err != nil {
    log.Println("Error creating file server: %v", err)
    return
  }
  http.HandleFunc("/", handle)
  http.ListenAndServe(":" + PORT, nil)
}

func handle(writer http.ResponseWriter, request *http.Request) {
  if strings.HasPrefix(request.URL.Path, "/foo/") {
    gFileServer.Handle(writer, request)
    return
  }
  sendError(writer, 500, "Internal Server Error: Could not handle.")
  return
}
```

The `MakeFileServer()` method creates a new `FileServer` instance. It has two arguments: `rootDir` and `urlPath`.

The `Handle()` method is the bread and butter here. If you call it in a `http.server.handle()` method, it will take care of handling the request. It does this by taking the `request.URL.Path` and replacing the `urlPath` prefix with `rootDir` to determine the path of the relevant file or directory. In this way, `rootDir` and `urlPath` indicate the top-level directory the user should have access to in the file system and URL path, respectively.

## Handle()

How does it handle requests? Here is a table of the supported HTTP methods:

| Method | Example URL path        | Description
| -------|-------------------------|-------------------------------------------------|
| GET    | /url-prefix/foo/bar.jpg |  Fetch the requested file or directory          |
| HEAD   | /url-prefix/foo/bar.jpg |  Like GET but with just the appropriate headers |
| PUT    | /url-prefix/foo/bar.jpg |  Save the request body as a new file.           |
| DELETE | /url-prefix/foo/bar.jpg |  Delete the file or directory.                  |
| POST   | /url-prefix/foo         |  Upload files from a form to a directory.       |
| PATCH  | /url-prefix/foo/bar.jpg |  Perform other actions.                         |

Most of these deserve some elaboration:
* `GET` - If a directory is requested, a list of the relevant file names are returned
* `HEAD` - This specifically returns the appropriate `Content-Type` and `Content-Length` headers.
* `PUT` - If the file already exists or requires making directories, the request will fail.
* `DELETE` - Directories are deleted recursively. Note that an attempt to delete the root directory (i.e. `/url-prefix/`) will succeed but will immediately re-create a new empty root directory.


`PATCH` deserves much more elaboration.

When handling such a request, the server expects a JSON request body with two fields:

| Key       | Value Type
| --------- | -------------------------------------------------------------------- |
| Command   | A string                                                             |
| OtherPath | A string representing a path. Required by some but not all commands. |

There are 9 supported commands:

| Command | OtherPath | Description                                                    |
| ------- | --------- | -------------------------------------------------------------- |
| -d      | Ignored   | Return `"1"` if `Path` is a directory. Otherwise returns `""`. |
| mv      | Required  | Move an entity from `Path` to `OtherPath`. (a)                 |
| cp      | Required  | Copy an entity from `Path` to `OtherPath` (a)                  |
| zip     | Required  | Zip an entity from `Path` into `OtherPath` (a)                 |
| unzip   | Required  | Unzip a file from `Path` to `OtherPath` (a) (d)                |
| ls      | Ignored   | List entities in a directory (c)                               |
| mkdir   | Ignored   | Create a directory (a)                                         |
| md5     | Ignored   | Return the md5 checksum of the file at Path (b)                |
| sha256  | Ignored   | Return the sha256 checksum of the file at Path (b)             |

(a) This command will not create neccessary ancestors and will fail if anything else is already at the destination.
(b) This command will fail on directories.
(c) This command will fail on files.
(d) It is unclear how this command will handle a directory. TODO: Fix this.


This server supports multi-threading as follows:
* All requests are entered into a FIFO queue.
* The highest-priority request in the queue waits to be processed until no other request is affecting the files or directories it will read or write.
* All requests of lower priority wait to be processed.

In the future, we expect to make this smarter by allowing requests to skip if they will have no effect on higher-priority requests.

## Locking

Internally, `FileServer` makes sure multiple requests don't try to read and write to the same files and directories at the same time. However, on its own, it can't stop *you* from writing code that reads and writes to files it needs. To avoid issues, it provides two more methods:

| Method                       |
| -----------------------------|
| `Lock(paths []string) error` |
| `Unlock(paths []string)`     |

An "entity" is specified by the path to a file or directory relative to the `rootDir` discussed above.

These are both synchronous. When you lock an entity, `FileServer` guarantees it won't read from or write to it until you call `Unlock`. Locking a directory also locks all its descendants.

## FileUtil

`FileUtil.py` consists of a single Python utility class of the same name. It provides clients with a convenient way to interface with a server like the one at the top of this README.

Here is some sample usage:

```python
from utils import FileUtil

fu = FileUtil('http://localhost:8080', '/url-prefix/', {'password': 'optional_password'})

# Wipe everything.
fu.delete('')

fu.mkdir('foo')

# Upload bar1.png.
fu.put(fu.dataFromFile('bar1.png'), 'foo/bar2.png')

# Download the same image.
fu.fileFromData(fu.get('foo/bar2.png'), 'bar3.png')

# Some additional methods:
fu.isDir('foo') # Returns True
fu.isDir('baz') # Returns False
fu.isDir('foo/bar2.png') # Returns False

fu.mv('foo/bar2.png', 'foo/bar4.png')
fu.cp('foo/bar4.png', 'foo/bar2.png')
fu.zip('foo/bar2.png', 'foo/bar2.zip')
fu.unzip('foo/bar2.zip', 'foo/bar5.png')
fu.ls('foo') # Returns ['bar2.png', 'bar4.png', 'bar2.zip', 'bar5.png']

fu.md5('foo2.png')    # Returns the md5 hash string
fu.sha256('foo2.png') # Returns the sha256 hash string

fu.delete('foo') # Delete everything we just did.
```

It's worth discussing some issues that arise when dealing with large files. There are four methods that create new files:

| Method                           | Description                      |
| -------------------------------- | -------------------------------- |
| get(url_path_from)               | Get the data of a small file.    |
| put(data, url_path_to)           | Upload the data of a small file. |
| download(url_path_from, path_to) | Download any file                |
| upload(path_from, url_path_to)   | Upload any file                  |

Notes:
* The first two methods give/take the *data* of a file. They both only consistently work for files less than 10 MB.
* The last two methods give/take *file paths*. They both consistently work for files exceeding a GB.
