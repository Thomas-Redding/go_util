package fileScheduler

import (
  "strings"
  "sync"
  "time"
)

/********** FileScheduler **********/

type FileScheduler struct {
  fileLocker FileLocker
  channel chan *Task
  priorityQueue TaskPriorityQueue
  routinePaths map[int64][][]string
  enqueuedRoutines map[int64]bool
  counterLock sync.Mutex
  counter int64
}

func MakeFileScheduler() FileScheduler {
  rtn := FileScheduler{
    fileLocker: MakeFileLocker(),
    channel: make(chan *Task, 0),
    priorityQueue: MakeTaskPriorityQueue(),
    routinePaths: make(map[int64][][]string),
    enqueuedRoutines: make(map[int64]bool),
    counter: 0,
  }

  // Add tasks to the priority queue.
  // Only this channel is allowed to use rtn.enqueuedRoutines
  go func() {
    for task := range rtn.channel {
      _, ok := rtn.enqueuedRoutines[task.RoutineId]
      if task.IsComplete {
        if ok {
          delete(rtn.enqueuedRoutines, task.RoutineId)
        } else {
          // Error: This routine has already released all its resources.
          continue
        }
      } else {
        if ok {
          // Error: This routine already has locked items. To prevent deadlock, give up.
          task.Channel <- false
          continue
        } else {
          rtn.enqueuedRoutines[task.RoutineId] = true
        }
      }
      rtn.priorityQueue.Push(task)
    }
  }()
  go func() {
    counter := 0
    for {
      counter = (counter + 1) % 1000
      time.Sleep(time.Millisecond)
      // Now process tasks:
      for rtn.priorityQueue.Length() != 0 {
        task := rtn.priorityQueue.Peek().(*Task)
        if !task.IsComplete {
          // Task needs to be done. Attempt to acquire locks.
          blocked := false
          for _, path := range task.Paths {
            isLocked := rtn.fileLocker.Locked(path, task.RoutineId)
            if isLocked {
              // File is locked by a different routine
              blocked = true
              break
            }
          }
          if blocked {
            break
          }
          rtn.routinePaths[task.RoutineId] = task.Paths
          for _, path := range task.Paths {
            rtn.fileLocker.Lock(path, task.RoutineId)
          }
          task.Channel <- true // Tell task to start
          rtn.priorityQueue.Pop()
        } else {
          // Task is done. Release all locks.
          for _, path := range rtn.routinePaths[task.RoutineId] {
            err := rtn.fileLocker.Unlock(path, task.RoutineId)
            if err != nil {
              // TODO: Log
            }
          }
          delete(rtn.routinePaths, task.RoutineId)
          rtn.priorityQueue.Pop()
        }
      }
    }
  }()
  return rtn
}

func (fileScheduler *FileScheduler) Lock(path string) int64 {
  return fileScheduler.lock([]string{path})
}

func (fileScheduler *FileScheduler) LockAll(paths []string) int64 {
  return fileScheduler.lock(paths)
}

func (fileScheduler *FileScheduler)lock(paths []string) int64 {
  pathArray := make([][]string, len(paths))
  for i, path := range paths {
    pathArray[i] = strings.Split(path, "/")
  }
  priority := time.Now().UnixNano()
  fileScheduler.counterLock.Lock()
  fileScheduler.counter += 1
  id := fileScheduler.counter
  fileScheduler.counterLock.Unlock()
  task := MakeTask(id, pathArray, priority, false)
  fileScheduler.channel <- task
  didSucceed := <- task.Channel // wait to lock files
  if didSucceed {
    return id
  } else {
    return 0
  }
}

func (fileScheduler *FileScheduler) Unlock(id int64) {
  task := MakeTask(id, nil, -1, true)
  task.IsComplete = true
  fileScheduler.channel <- task // no need to wait for files to become unlocked
}
