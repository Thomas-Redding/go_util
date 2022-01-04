
import os
import json
import requests
import shutil

class FileUtil:
  def __init__(self, baseURL, urlPrefix, cookies):
    assert (baseURL.startswith('http://') and '/' not in baseURL[len('http://'):]) or (baseURL.startswith('https://') and '/' not in baseURL[len('https://'):])
    assert ':' not in urlPrefix
    assert urlPrefix.endswith('/')
    self._baseURL = baseURL
    self._urlPrefix = urlPrefix
    self._cookies = cookies
    self._lastRequest = None

  # Read the bytes from a local file.
  def dataFromFile(self, path):
    with open(path, 'rb') as f:
      data = f.read()
    return data

  # Write bytes into a new local file.
  def fileFromData(self, data, path):
    with open(path, 'wb') as f:
      f.write(data)

  # Download a file from the server to memory.
  # Only works reliabily for files less than 10 mb.
  def get(self, url_path_from):
    r = requests.get(self._baseURL + self._urlPrefix + url_path_from, cookies=self._cookies)
    self._lastRequest = r
    assert r.status_code == 200
    return r.content

  # Upload a local file to the server.
  def put(self, data, url_path_to):
    r = requests.put(self._baseURL + self._urlPrefix + url_path_to, data=data, cookies=self._cookies)
    self._lastRequest = r
    assert r.status_code == 200

  # Delete a file on the server.
  def delete(self, url_path):
    r = requests.delete(self._baseURL + self._urlPrefix + url_path, cookies=self._cookies)
    self._lastRequest = r
    assert r.status_code == 200

  # Download a file from the server to local disk.
  # Works for all file sizes.
  def download(self, url_path_from, path_to):
    # https://stackoverflow.com/a/39217788/4004969
    with requests.get(self._baseURL + self._urlPrefix + url_path_from, cookies=self._cookies, stream=True) as r:
      with open(path_to, 'wb') as f:
        shutil.copyfileobj(r.raw, f)
      self._lastRequest = r
    assert r.status_code == 200, (r.status_code, '?')

  # Upload a local file to the server.
  def upload(self, path_from, url_path_to):
    with open(path_from, 'rb') as f:
      r = requests.post(self._baseURL + self._urlPrefix + os.path.dirname(url_path_to), files={os.path.basename(url_path_to): f}, cookies=self._cookies)
    self._lastRequest = r
    assert r.status_code == 200, (r.status_code, r.text)

  def _patch(self, url_path, body):
    if 'otherPath' in body:
      body['otherPath'] = self._urlPrefix + body['otherPath']
    r = requests.patch(self._baseURL + self._urlPrefix + url_path, data=bytes(json.dumps(body), encoding="utf-8"), cookies=self._cookies)
    self._lastRequest = r
    assert r.status_code == 200
    return r

  def isDir(self, url_path):
    return len(self._patch(url_path, {'command': '-d'}).text) > 0

  def mv(self, url_path_from, url_path_to):
    self._patch(url_path_from, {'command': 'mv', 'otherPath': url_path_to})

  def cp(self, url_path_from, url_path_to):
    self._patch(url_path_from, {'command': 'cp', 'otherPath': url_path_to})

  def zip(self, url_path_from, url_path_to):
    assert url_path_to.endswith('.zip')
    self._patch(url_path_from, {'command': 'zip', 'otherPath': url_path_to})

  def unzip(self, url_path_from, url_path_to):
    assert url_path_from.endswith('.zip')
    self._patch(url_path_from, {'command': 'unzip', 'otherPath': url_path_to})

  def ls(self, url_path):
    response = self._patch(url_path, {'command': 'ls'})
    return response.text.split('\n')

  def mkdir(self, url_path):
    self._patch(url_path, {'command': 'mkdir'})

  def md5(self, url_path):
    response = self._patch(url_path, {'command': 'md5'})
    return response.text

  def sha256(self, url_path):
    response = self._patch(url_path, {'command': 'sha256'})
    return response.text
