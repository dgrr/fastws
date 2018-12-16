package fastws

import "sync"

type locker struct {
	b   bool
	cnd *sync.Cond
}

func newLocker() *locker {
	lck := &locker{}
	lck.init()
	return lck
}

func (lck *locker) init() {
	lck.b = false
	lck.cnd = sync.NewCond(&sync.Mutex{})
}

func (lck *locker) Lock() {
	lck.cnd.L.Lock()
	for lck.b {
		lck.cnd.Wait()
	}
	lck.b = true
	lck.cnd.L.Unlock()
}

func (lck *locker) Unlock() {
	lck.b = false
	lck.cnd.Signal()
}
