package fileScheduler

import (
  "strings"
  "time"
)

/*
 * FileScheduler supports recursive locks, but every lock must be matched by an unlock.
 */

/********** FileScheduler **********/

type FileScheduler struct {
  fileLocker FileLocker
  channel chan *Task
  priorityQueue TaskPriorityQueue
  routinePaths map[string][][]string
  enqueuedRoutines map[string]bool
}

func MakeFileScheduler() FileScheduler {
  rtn := FileScheduler{
    fileLocker: MakeFileLocker(),
    channel: make(chan *Task, 0),
    priorityQueue: MakeTaskPriorityQueue(),
    routinePaths: make(map[string][][]string),
    enqueuedRoutines: make(map[string]bool),
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

// TODO: Remove routineId parameter and use some identifier of the go-routine.
func (fileScheduler *FileScheduler) Lock(routineId string, path string) bool {
  return fileScheduler.LockAll(routineId, []string{path})
}

func (fileScheduler *FileScheduler) LockAll(routineId string, paths []string) bool {
  return fileScheduler.lock(routineId, paths, time.Now().UnixNano())
}

func (fileScheduler *FileScheduler) LockUrgent(routineId string, path string) bool {
  return fileScheduler.LockAllUrgent(routineId, []string{path})
}

func (fileScheduler *FileScheduler) LockAllUrgent(routineId string, paths []string) bool {
  return fileScheduler.lock(routineId, paths, time.Now().UnixNano() / 2)
}

func (fileScheduler *FileScheduler)lock(routineId string, paths []string, priority int64) bool {
  pathArray := make([][]string, len(paths))
  for i, path := range paths {
    pathArray[i] = strings.Split(path, "/")
  }
  task := MakeTask(routineId, pathArray, priority, false)
  fileScheduler.channel <- task
  didSucceed := <- task.Channel // wait to lock files
  return didSucceed
}

func (fileScheduler *FileScheduler) Unlock(routineId string) {
  task := MakeTask(routineId, nil, -1, true)
  task.IsComplete = true
  fileScheduler.channel <- task // no need to wait for files to become unlocked
}
